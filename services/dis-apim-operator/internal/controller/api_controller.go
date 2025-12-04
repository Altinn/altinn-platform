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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apimv1alpha1 "github.com/Altinn/altinn-platform/services/dis-apim-operator/api/v1alpha1"
)

var (
	jobOwnerKey = ".metadata.controller"
	apiGVStr    = apimv1alpha1.GroupVersion.String()
)

const API_FINALIZER = "api.apim.dis.altinn.cloud/finalizer"

// ApiReconciler reconciles a Api object
type ApiReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	NewClient        newApimClient
	ApimClientConfig *azure.ApimClientConfig
	apimClient       *azure.APIMClient
}

// +kubebuilder:rbac:groups=apim.dis.altinn.cloud,resources=apis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apim.dis.altinn.cloud,resources=apis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apim.dis.altinn.cloud,resources=apis/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Api object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.1/pkg/reconcile
func (r *ApiReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var api apimv1alpha1.Api
	if err := r.Get(ctx, req.NamespacedName, &api); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "unable to fetch Api")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !controllerutil.ContainsFinalizer(&api, API_FINALIZER) {
		controllerutil.AddFinalizer(&api, API_FINALIZER)
		err := r.Update(ctx, &api)
		if err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}
	if r.apimClient == nil {
		c, err := r.NewClient(r.ApimClientConfig)
		if err != nil {
			logger.Error(err, "Failed to create new APIM client")
			return ctrl.Result{}, err
		}
		r.apimClient = c
	}
	if api.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &api)
	}
	azApi, err := r.apimClient.GetApiVersionSet(ctx, api.GetApiAzureFullName(), nil)
	if err != nil {
		if azure.IsNotFoundError(err) {

			return r.handleCreateUpdate(ctx, &api)
		}
		logger.Error(err, "Failed to get Azure Apim Api Version Set")
		return ctrl.Result{}, err
	}
	if azApi.ID == nil {
		logger.Info("No ID returned for API")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}
	api.Status.ApiVersionSetID = *azApi.ID
	inSync, err := r.reconcileVersions(ctx, &api)
	if err != nil {
		logger.Error(err, "Failed to reconcile versions")
		return ctrl.Result{}, err
	}
	if !inSync {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	err = r.deleteRemovedVersions(ctx, &api)
	if err != nil {
		logger.Error(err, "Failed to delete removed versions")
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApiReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &apimv1alpha1.ApiVersion{}, jobOwnerKey, func(rawObj client.Object) []string {
		// grab the job object, extract the owner...
		job := rawObj.(*apimv1alpha1.ApiVersion)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		// ...make sure it's a CronJob...
		if owner.APIVersion != apiGVStr || owner.Kind != "Api" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&apimv1alpha1.Api{}).
		Owns(&apimv1alpha1.ApiVersion{}).
		WithEventFilter(defaultPredicate(r.ApimClientConfig.NamespaceSuffix)).
		Named("api").
		Complete(r)
}

func (r *ApiReconciler) handleDeletion(ctx context.Context, api *apimv1alpha1.Api) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	origApi := api.DeepCopy()
	patch := client.MergeFrom(origApi)
	azApiVS, err := r.apimClient.GetApiVersionSet(ctx, api.GetApiAzureFullName(), nil)
	if err != nil {
		if azure.IsNotFoundError(err) {
			logger.Info("API version set not found", "VersionSetId", api.GetApiAzureFullName())
			controllerutil.RemoveFinalizer(api, API_FINALIZER)
			return ctrl.Result{}, r.Update(ctx, api)
		}
		logger.Error(err, "Failed to get API version set", "VersionSetId", api.GetApiAzureFullName())
		return ctrl.Result{}, err
	}
	if r.isAllVersionsDeleted(ctx, api) {
		logger.Info("Deleting API version set", "VersionSetId", api.GetApiAzureFullName())
		eTag := "*"
		if azApiVS.ETag != nil {
			eTag = *azApiVS.ETag
		}
		_, err := r.apimClient.DeleteApiVersionSet(ctx, api.GetApiAzureFullName(), eTag, nil)
		if err != nil {
			logger.Error(err, "Failed to delete API version set", "VersionSetId", api.GetApiAzureFullName())
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(api, API_FINALIZER)
		return ctrl.Result{}, r.Update(ctx, api)
	}
	api.Status.ProvisioningState = apimv1alpha1.ProvisioningStateDeleting

	if err := r.handleDeleteVersions(ctx, api); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, r.Status().Patch(ctx, api, patch)
}

func (r *ApiReconciler) handleDeleteVersions(ctx context.Context, api *apimv1alpha1.Api) error {
	var apiVersionsList apimv1alpha1.ApiVersionList
	apiVersionErr := r.List(ctx, &apiVersionsList, client.InNamespace(api.Namespace), client.MatchingFields{jobOwnerKey: api.Name})
	if client.IgnoreNotFound(apiVersionErr) != nil {
		return apiVersionErr
	}
	versions := apiVersionsList.Items
	for _, version := range versions {
		if err := r.Delete(ctx, &version); client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

func (r *ApiReconciler) isAllVersionsDeleted(ctx context.Context, api *apimv1alpha1.Api) bool {
	logger := log.FromContext(ctx)
	var apiVersionList apimv1alpha1.ApiVersionList
	apiVersionErr := r.List(ctx, &apiVersionList, client.InNamespace(api.Namespace), client.MatchingFields{jobOwnerKey: api.Name})
	versions := apiVersionList.Items
	if client.IgnoreNotFound(apiVersionErr) != nil {
		logger.Error(apiVersionErr, "Failed to list versions")
		return false
	}
	if len(versions) > 0 {
		logger.Info("Versions still exist", "Count", len(versions))
		return false
	}
	logger.Info("All versions deleted")
	return true
}

func (r *ApiReconciler) handleCreateUpdate(ctx context.Context, api *apimv1alpha1.Api) (ctrl.Result, error) {
	orig := api.DeepCopy()
	patch := client.MergeFrom(orig)
	azApi, err := r.apimClient.CreateUpdateApiVersionSet(ctx, api.GetApiAzureFullName(), api.ToAzureApiVersionSet(), nil)
	if err != nil {
		return ctrl.Result{}, err
	}
	if azApi.ID == nil {
		return ctrl.Result{}, fmt.Errorf("no ID returned for API version set %s", api.GetApiAzureFullName())
	}
	api.Status.ApiVersionSetID = *azApi.ID
	api.Status.ProvisioningState = apimv1alpha1.ProvisioningStateUpdating
	err = r.Status().Patch(ctx, api, patch)
	return ctrl.Result{RequeueAfter: 10 * time.Second}, err
}

func (r *ApiReconciler) reconcileVersions(ctx context.Context, api *apimv1alpha1.Api) (bool, error) {
	logger := log.FromContext(ctx)
	wantedVersions := api.ToApiVersions()
	inSync := true
	origApi := api.DeepCopy()
	patch := client.MergeFrom(origApi)
	if api.Status.VersionStates == nil && len(wantedVersions) > 0 {
		api.Status.VersionStates = make(map[string]apimv1alpha1.ApiVersionStatus)
	}
	for k, wantedVersion := range wantedVersions {
		var existingVersion apimv1alpha1.ApiVersion
		if err := r.Get(ctx, client.ObjectKey{Namespace: api.Namespace, Name: wantedVersion.Name}, &existingVersion); err != nil {
			if client.IgnoreNotFound(err) != nil {
				logger.Error(err, "Failed to get ApiVersion")
				return false, err
			}
			if err := controllerutil.SetControllerReference(api, &wantedVersion, r.Scheme); err != nil {
				logger.Error(err, "Failed to set controller reference")
				return false, err
			}
			if err := r.Create(ctx, &wantedVersion); err != nil {
				logger.Error(err, "Failed to create ApiVersion")
				return false, err
			}
			inSync = false
			continue
		} else {
			if !wantedVersion.Matches(existingVersion) {
				origVersion := existingVersion.DeepCopy()
				patch := client.MergeFrom(origVersion)
				existingVersion.Spec = wantedVersion.Spec
				if err := r.Patch(ctx, &existingVersion, patch); err != nil {
					logger.Error(err, "Failed to update ApiVersion")
					return false, err
				}
				inSync = false
				api.Status.VersionStates[k] = existingVersion.Status
			} else {
				if existingVersion.Status.ProvisioningState != apimv1alpha1.ProvisioningStateSucceeded {
					inSync = false
				}
				api.Status.VersionStates[k] = existingVersion.Status
			}
		}
	}
	if !inSync {
		api.Status.ProvisioningState = getStatusFromVersionStatuses(api.Status.VersionStates)
	} else {
		api.Status.ProvisioningState = apimv1alpha1.ProvisioningStateSucceeded
	}
	return inSync, r.Status().Patch(ctx, api, patch)
}

func (r *ApiReconciler) deleteRemovedVersions(ctx context.Context, api *apimv1alpha1.Api) error {
	origApi := api.DeepCopy()
	patch := client.MergeFrom(origApi)
	var apiVersionList apimv1alpha1.ApiVersionList
	err := r.List(ctx, &apiVersionList, client.InNamespace(api.Namespace), client.MatchingFields{jobOwnerKey: api.Name})
	if err != nil {
		return err
	}
	versions := apiVersionList.Items
	if len(versions) == len(api.Spec.Versions) || len(versions) == 0 {
		return nil
	}

	for _, version := range versions {
		if !versionInList(version, api.Name, api.Spec.Versions) {
			if err := r.Delete(ctx, &version); client.IgnoreNotFound(err) != nil {
				return err
			}
			delete(api.Status.VersionStates, *version.Spec.Name)
		}
	}
	return r.Status().Patch(ctx, api, patch)
}

func versionInList(version apimv1alpha1.ApiVersion, apiName string, versions []apimv1alpha1.ApiVersionSubSpec) bool {
	for _, v := range versions {
		if v.GetApiVersionFullName(apiName) == version.Name {
			return true
		}
	}
	return false
}

func getStatusFromVersionStatuses(versions map[string]apimv1alpha1.ApiVersionStatus) apimv1alpha1.ProvisioningState {
	state := apimv1alpha1.ProvisioningStateSucceeded
	for _, v := range versions {
		if v.ProvisioningState == apimv1alpha1.ProvisioningStateFailed {
			return apimv1alpha1.ProvisioningStateFailed
		}
		if v.ProvisioningState != apimv1alpha1.ProvisioningStateSucceeded {
			state = apimv1alpha1.ProvisioningStateUpdating
		}
	}
	return state
}
