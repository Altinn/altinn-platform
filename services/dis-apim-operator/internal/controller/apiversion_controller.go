/*
Copyright 2024 altinn.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/azure"
	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/utils"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v3"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apimv1alpha1 "github.com/Altinn/altinn-platform/services/dis-apim-operator/api/v1alpha1"
)

const (
	API_VERSION_FINALIZER      = "apiversion.apim.dis.altinn.cloud/finalizer"
	DEFAULT_REQUE_TIME         = 1 * time.Minute
	WAITING_FOR_LRO_REQUE_TIME = 5 * time.Second
)

// ApiVersionReconciler reconciles a ApiVersion object
type ApiVersionReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	NewClient        newApimClient
	ApimClientConfig *azure.ApimClientConfig
	apimClient       *azure.APIMClient
}

// +kubebuilder:rbac:groups=apim.dis.altinn.cloud,resources=apiversions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apim.dis.altinn.cloud,resources=apiversions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apim.dis.altinn.cloud,resources=apiversions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ApiVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ApiVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var apiVersion apimv1alpha1.ApiVersion
	if err := r.Get(ctx, req.NamespacedName, &apiVersion); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "unable to fetch ApiVersion")
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !controllerutil.ContainsFinalizer(&apiVersion, API_VERSION_FINALIZER) {
		controllerutil.AddFinalizer(&apiVersion, API_VERSION_FINALIZER)
		err := r.Update(ctx, &apiVersion)
		if err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}
	if r.apimClient == nil {
		c, err := r.NewClient(r.ApimClientConfig)
		if err != nil {
			logger.Error(err, "Failed to create new client")
			return ctrl.Result{}, err
		}
		r.apimClient = c
	}
	if !apiVersion.DeletionTimestamp.IsZero() {
		return r.deleteApiVersion(ctx, apiVersion)
	}
	_, err := r.apimClient.GetApi(ctx, apiVersion.GetApiVersionAzureFullName(), nil)
	if azure.IgnoreNotFound(err) != nil {
		logger.Error(err, "Failed to get API")
		return ctrl.Result{}, err
	} else {
		return r.handleApiVersionUpdate(ctx, apiVersion)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApiVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apimv1alpha1.ApiVersion{}).
		WithEventFilter(defaultPredicate(r.ApimClientConfig.NamespaceSuffix)).
		Named("apiversion").
		Complete(r)
}

func (r *ApiVersionReconciler) deleteApiVersion(ctx context.Context, apiVersion apimv1alpha1.ApiVersion) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Deleting APIVersion")
	if apiVersion.Status.ProvisioningState == apimv1alpha1.ProvisioningStateDeleted {
		if apiVersion.Spec.Policies != nil {
			_, err := r.apimClient.DeleteApiPolicy(ctx, apiVersion.GetApiVersionAzureFullName(), "*", nil)
			if azure.IgnoreNotFound(err) != nil {
				logger.Error(err, "Failed to delete policy")
				return ctrl.Result{}, err
			}
		}
		controllerutil.RemoveFinalizer(&apiVersion, API_VERSION_FINALIZER)
		err := r.Update(ctx, &apiVersion)
		if err != nil {
			logger.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	resumeToken := apiVersion.Status.ResumeToken
	options := &apim.APIClientBeginDeleteOptions{ResumeToken: resumeToken}
	poller, err := r.apimClient.DeleteApi(ctx, apiVersion.GetApiVersionAzureFullName(), "*", options)
	if err != nil {
		orig := apiVersion.DeepCopy()
		patch := client.MergeFrom(orig)
		if azure.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to delete APIVersion")
			return ctrl.Result{}, err
		}
		logger.Info("APIVersion deleted successfully")
		apiVersion.Status.ResumeToken = ""
		apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateDeleted
		apiVersion.Status.LastAppliedSpecSha = ""
		apiVersion.Status.LastAppliedPolicyBase64 = ""
		err = r.Status().Patch(ctx, &apiVersion, patch)
		if err != nil {
			logger.Error(err, "Failed to update status after deletion")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	logger.Info(fmt.Sprintf("Watching LR operation for deletion, resume-token: %s", resumeToken))
	status, _, token, err := azure.StartResumeOperation[apim.APIClientDeleteResponse](ctx, poller)
	if err != nil {
		logger.Error(err, "Failed to watch LR operation for deletion")
		return ctrl.Result{}, err
	}
	orig := apiVersion.DeepCopy()
	patch := client.MergeFrom(orig)
	switch status {
	case azure.OperationStatusFailed:
		logger.Error(err, "Failed to delete APIVersion")
		apiVersion.Status.ResumeToken = ""
		apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateFailed
		err = r.Status().Patch(ctx, &apiVersion, patch)
		if err != nil {
			logger.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, fmt.Errorf("failed to delete APIVersion: %w", err)
	case azure.OperationStatusSucceeded:
		logger.Info("APIVersion deleted successfully")
		apiVersion.Status.ResumeToken = ""
		apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateDeleted
		apiVersion.Status.LastAppliedSpecSha = ""
		apiVersion.Status.LastAppliedPolicyBase64 = ""
		err = r.Status().Patch(ctx, &apiVersion, patch)
		if err != nil {
			logger.Error(err, "Failed to update status after deletion")
			return ctrl.Result{}, err
		}
	case azure.OperationStatusInProgress:
		logger.Info("Deletion in progress, updating status")
		apiVersion.Status.ResumeToken = token
		apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateDeleting
		err = r.Status().Patch(ctx, &apiVersion, patch)
		if err != nil {
			logger.Error(err, "Failed to update status during deletion")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: WAITING_FOR_LRO_REQUE_TIME}, nil
	}
	return ctrl.Result{}, nil
}

func (r *ApiVersionReconciler) handleApiVersionUpdate(ctx context.Context, apiVersion apimv1alpha1.ApiVersion) (ctrl.Result, error) {
	latestSha, err := utils.Sha256FromContent(ctx, apiVersion.Spec.Content)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get api spec sha: %w", err)
	}
	if apiVersion.Status.LastAppliedSpecSha != latestSha {
		return r.createUpdateApimApi(ctx, apiVersion)
	}
	if apiVersion.Spec.Policies != nil {
		_, policyErr := r.apimClient.GetApiPolicy(ctx, apiVersion.GetApiVersionAzureFullName(), nil)
		if azure.IgnoreNotFound(policyErr) != nil {
			return ctrl.Result{}, policyErr
		}
		lastPolicySha, shaErr := utils.Sha256FromContent(ctx, apiVersion.Spec.Policies.PolicyContent)
		if shaErr != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get policy sha: %w", shaErr)
		}
		if apiVersion.Status.LastAppliedPolicySha != lastPolicySha || azure.IsNotFoundError(policyErr) {
			if err := r.createUpdatePolicy(ctx, apiVersion); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to create/update policy: %w", err)
			}
		}
	} else {
		if err = r.ensurePoliciesDeleted(ctx, apiVersion); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete policy: %w", err)
		}
	}
	if apiVersion.Spec.Diagnostics != nil {
		if err = r.createUpdateDiagnostics(ctx, apiVersion); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create/update Diagnostics: %w", err)
		}
	} else {
		if err = r.ensureDiagnosticsDeleted(ctx, apiVersion); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete Diagnostics: %w", err)
		}
	}
	return ctrl.Result{RequeueAfter: DEFAULT_REQUE_TIME}, nil
}

func (r *ApiVersionReconciler) createUpdateApimApi(ctx context.Context, apiVersion apimv1alpha1.ApiVersion) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	resumeToken := apiVersion.Status.ResumeToken
	logger.Info("Creating or updating API")
	apimApiParams := apiVersion.ToAzureCreateOrUpdateParameter()
	poller, err := r.apimClient.CreateUpdateApi(
		ctx,
		apiVersion.GetApiVersionAzureFullName(),
		apimApiParams,
		&apim.APIClientBeginCreateOrUpdateOptions{ResumeToken: resumeToken})

	if err != nil {
		logger.Error(err, "Failed to create/update API")
		apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateFailed
		_ = r.Status().Update(ctx, &apiVersion)
		return ctrl.Result{}, err
	}
	logger.Info("Watching LR operation")
	status, _, token, err := azure.StartResumeOperation[apim.APIClientCreateOrUpdateResponse](ctx, poller)
	if err != nil {
		logger.Error(err, "Failed to watch LR operation")
		return ctrl.Result{}, err
	}

	switch status {
	case azure.OperationStatusFailed:
		logger.Error(err, "Failed to watch LR operation")
		apiVersion.Status.ResumeToken = ""
		apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateFailed
		err = r.Status().Update(ctx, &apiVersion)
		if err != nil {
			logger.Error(err, "Failed to update status")
		}
		return ctrl.Result{}, err
	case azure.OperationStatusInProgress:
		apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateUpdating
		apiVersion.Status.ResumeToken = token
		err = r.Status().Update(ctx, &apiVersion)
		if err != nil {
			logger.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: WAITING_FOR_LRO_REQUE_TIME}, nil
	case azure.OperationStatusSucceeded:
		logger.Info("Operation completed")
		apiVersion.Status.ResumeToken = ""
		apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateSucceeded
		apiVersion.Status.LastAppliedSpecSha, err = utils.Sha256FromContent(ctx, apiVersion.Spec.Content)
		if apiVersion.Spec.Policies != nil {
			apiVersion.Status.LastAppliedPolicyBase64, err = utils.Base64FromContent(ctx, apiVersion.Spec.Policies.PolicyContent)
		}
		if err != nil {
			logger.Error(err, "Failed to get spec sha")
			return ctrl.Result{}, err
		}
		err = r.Status().Update(ctx, &apiVersion)
		if err != nil {
			logger.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: DEFAULT_REQUE_TIME}, nil
	default:
		logger.Error(nil, "Unexpected operation status", "status", status)
		return ctrl.Result{}, fmt.Errorf("unexpected operation status: %s", status)
	}
}

func (r *ApiVersionReconciler) createUpdatePolicy(ctx context.Context, apiVersion apimv1alpha1.ApiVersion) error {
	logger := log.FromContext(ctx)
	logger.Info("Creating or updating policy")
	orig := apiVersion.DeepCopy()
	patch := client.MergeFrom(orig)
	policy := apiVersion.Spec.Policies
	if policy.PolicyContent == nil {
		return fmt.Errorf("policy content is nil")
	}
	policyContent, err := r.runPolicyTemplating(ctx, policy.PolicyValues, *policy.PolicyContent, apiVersion.Namespace)
	if err != nil {
		return fmt.Errorf("failed to run policy templating: %w", err)
	}
	policyFormat := policy.PolicyFormat.AzurePolicyFormat()
	_, err = r.apimClient.CreateUpdateApiPolicy(
		ctx,
		apiVersion.GetApiVersionAzureFullName(),
		apim.PolicyContract{
			Properties: &apim.PolicyContractProperties{
				Value:  &policyContent,
				Format: policyFormat,
			}},
		nil,
	)
	if err != nil {
		logger.Error(err, "Failed to create/update policy")
		apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateFailed
		_ = r.Status().Patch(ctx, &apiVersion, patch)
		return err
	}
	apiVersion.Status.LastAppliedPolicySha, err = utils.Sha256FromContent(ctx, apiVersion.Spec.Policies.PolicyContent)
	if err != nil {
		logger.Error(err, "Failed to get policy sha")
		return err
	}
	apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateSucceeded
	err = r.Status().Patch(ctx, &apiVersion, patch)
	if err != nil {
		logger.Error(err, "Failed to update status")
		return err
	}
	return nil
}

func (r *ApiVersionReconciler) createUpdateDiagnostics(ctx context.Context, apiVersion apimv1alpha1.ApiVersion) error {
	logger := log.FromContext(ctx)
	orig := apiVersion.DeepCopy()
	patch := client.MergeFrom(orig)
	diagnostics := apiVersion.Spec.Diagnostics
	loggerId := r.ApimClientConfig.DefaultLoggerId
	azuremonitorLoggerId := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ApiManagement/service/%s/loggers/azuremonitor", r.ApimClientConfig.SubscriptionId, r.ApimClientConfig.ResourceGroup, r.ApimClientConfig.ApimServiceName)
	if diagnostics.LoggerName != nil {
		lookedUpId, err := r.apimClient.GetLoggerByName(ctx, *diagnostics.LoggerName)
		if err != nil {
			return fmt.Errorf("failed to get logger: %w", err)
		}
		if lookedUpId == nil {
			return fmt.Errorf("logger not found")
		}
		loggerId = *lookedUpId
	}
	_, err := r.apimClient.CreateUpdateApiDiagnosticSettings(
		ctx,
		apiVersion.GetApiVersionAzureFullName(),
		azure.DiagnosticsIdApplicationInsights,
		apiVersion.GetAzureAPIAppInsightsDiagnosticSettings(loggerId),
		nil,
	)
	if err != nil {
		logger.Error(err, "Failed to create/update appinsights diagnostics")
		apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateFailed
		_ = r.Status().Patch(ctx, &apiVersion, patch)
		return err
	}
	_, err = r.apimClient.CreateUpdateApiDiagnosticSettings(
		ctx,
		apiVersion.GetApiVersionAzureFullName(),
		azure.DiagnosticsIdAzureMonitor,
		apiVersion.GetAzureAPIAzureMonitorDiagnosticSettings(azuremonitorLoggerId),
		nil,
	)
	if err != nil {
		logger.Error(err, "Failed to create/update azuremonitor diagnostics")
		apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateFailed
		_ = r.Status().Patch(ctx, &apiVersion, patch)
		return err
	}
	apiVersion.Status.ProvisioningState = apimv1alpha1.ProvisioningStateSucceeded
	err = r.Status().Patch(ctx, &apiVersion, patch)
	if err != nil {
		logger.Error(err, "Failed to update status")
		return err
	}
	return nil
}

func (r *ApiVersionReconciler) ensurePoliciesDeleted(ctx context.Context, apiVersion apimv1alpha1.ApiVersion) error {
	logger := log.FromContext(ctx)
	policy, err := r.apimClient.GetApiPolicy(ctx, apiVersion.GetApiVersionAzureFullName(), nil)
	if azure.IgnoreNotFound(err) != nil {
		logger.Error(err, "Failed to get policy")
		return err
	}
	if err == nil {
		_, err := r.apimClient.DeleteApiPolicy(ctx, *policy.Name, *policy.ETag, nil)
		if azure.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to delete policy")
			return err
		}
	}
	return nil
}

func (r *ApiVersionReconciler) ensureDiagnosticsDeleted(ctx context.Context, apiVersion apimv1alpha1.ApiVersion) error {
	logger := log.FromContext(ctx)
	appInsightsDiag, err := r.apimClient.GetApiDiagnosticSettings(ctx, apiVersion.GetApiVersionAzureFullName(), string(azure.DiagnosticsIdApplicationInsights), nil)
	if azure.IgnoreNotFound(err) != nil {
		logger.Error(err, "Failed to get appinsights diagnostics")
		return err
	}
	if err == nil {
		_, err = r.apimClient.DeleteApiDiagnosticSettings(ctx, apiVersion.GetApiVersionAzureFullName(), *appInsightsDiag.Name, *appInsightsDiag.ETag, nil)
		if err != nil {
			logger.Error(err, "Failed to delete appinsights diagnostics")
			return err
		}
	}
	azureMonitorDiag, err := r.apimClient.GetApiDiagnosticSettings(ctx, apiVersion.GetApiVersionAzureFullName(), string(azure.DiagnosticsIdAzureMonitor), nil)
	if azure.IgnoreNotFound(err) != nil {
		logger.Error(err, "Failed to get azuremonitor diagnostics")
		return err
	}
	if err == nil {
		_, err = r.apimClient.DeleteApiDiagnosticSettings(ctx, apiVersion.GetApiVersionAzureFullName(), *azureMonitorDiag.Name, *azureMonitorDiag.ETag, nil)
		if err != nil {
			logger.Error(err, "Failed to delete azuremonitor diagnostics")
			return err
		}
	}
	return nil
}

func (r *ApiVersionReconciler) runPolicyTemplating(ctx context.Context, values []apimv1alpha1.PolicyValue, policyContent string, apiVersionNamespace string) (string, error) {
	data := make(map[string]string)
	for _, v := range values {
		if v.IdFromBackend != nil {
			namespace := apiVersionNamespace
			if v.IdFromBackend.Namespace != nil {
				namespace = *v.IdFromBackend.Namespace
			}
			var backend apimv1alpha1.Backend
			err := r.Get(ctx, client.ObjectKey{Name: v.IdFromBackend.Name, Namespace: namespace}, &backend)
			if err != nil {
				return "", fmt.Errorf("failed to get backend: %w", err)
			}
			data[v.Name] = backend.GetAzureResourceName()
			continue
		}
		if v.Value != nil {
			data[v.Name] = *v.Value
		}
	}
	return utils.GeneratePolicyFromTemplate(policyContent, data)
}
