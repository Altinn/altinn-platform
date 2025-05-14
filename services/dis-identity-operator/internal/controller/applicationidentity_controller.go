/*
Copyright 2025 Altinn.

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
	"time"

	managedidentity "github.com/Azure/azure-service-operator/v2/api/managedidentity/v1api20230131"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	applicationv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-identity-operator/internal/config"
)

const applicationIdentityFinalizer = "applicationidentity.application.dis.altinn.cloud/finalizer"

// ApplicationIdentityReconciler reconciles a ApplicationIdentity object
type ApplicationIdentityReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.DisIdentityConfig
}

// +kubebuilder:rbac:groups=application.dis.altinn.cloud,resources=applicationidentities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=application.dis.altinn.cloud,resources=applicationidentities/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=application.dis.altinn.cloud,resources=applicationidentities/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ApplicationIdentity object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *ApplicationIdentityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	// Fetch the ApplicationIdentity instance
	applicationIdentity := &applicationv1alpha1.ApplicationIdentity{}
	if err := r.Get(ctx, req.NamespacedName, applicationIdentity); err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "unable to fetch ApplicationIdentity")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Set finalizer for the ApplicationIdentity instance if it doesn't exist
	if !controllerutil.ContainsFinalizer(applicationIdentity, applicationIdentityFinalizer) {
		controllerutil.AddFinalizer(applicationIdentity, applicationIdentityFinalizer)
		if err := r.Update(ctx, applicationIdentity); err != nil {
			logger.Error(err, "unable to update ApplicationIdentity with finalizer")
			return ctrl.Result{}, err
		}
	}
	// Check if the UserAssignedIdentity already exists

	// Check if the ApplicationIdentity instance is marked to be deleted
	if applicationIdentity.GetDeletionTimestamp() != nil {
		// Verify that the UserAssignedIdentity is deleted
		uaIDRemoved, err := r.removeUserAssignedIdentity(ctx, applicationIdentity)
		if err != nil {
			logger.Error(err, "unable to remove UserAssignedIdentity")
			return ctrl.Result{}, err
		}

		if !uaIDRemoved {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		// Remove the finalizer from the ApplicationIdentity instance
		controllerutil.RemoveFinalizer(applicationIdentity, applicationIdentityFinalizer)
		if err := r.Update(ctx, applicationIdentity); err != nil {
			logger.Error(err, "unable to update ApplicationIdentity with finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	uaID := &managedidentity.UserAssignedIdentity{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      applicationIdentity.Name,
		Namespace: applicationIdentity.Namespace,
	}, uaID)
	// Check UserAssignedIdentity status
	uaIDReady := false
	if client.IgnoreNotFound(err) != nil {
		logger.Error(err, "unable to fetch UserAssignedIdentity")
		return ctrl.Result{}, err
	} else if errors.IsNotFound(err) {
		return ctrl.Result{}, r.createNewUserAssignedIdentity(ctx, applicationIdentity)
	} else {
		uaIDReady, err = r.updateUserAssignedIdentityStatus(ctx, applicationIdentity, uaID)
		if err != nil {
			logger.Error(err, "unable to update ApplicationIdentity status")
			return ctrl.Result{}, err
		}
	}
	if !uaIDReady {
		return ctrl.Result{}, nil
	}

	// Check FederatedIdentityCredential status
	fedCredsReady := false
	fedCreds := &managedidentity.FederatedIdentityCredential{}
	err = r.Get(ctx, client.ObjectKey{
		Name:      applicationIdentity.Name,
		Namespace: applicationIdentity.Namespace,
	}, fedCreds)
	if client.IgnoreNotFound(err) != nil {
		logger.Error(err, "unable to fetch FederatedIdentityCredential")
		return ctrl.Result{}, err
	} else if errors.IsNotFound(err) {
		return ctrl.Result{}, r.createFederation(ctx, applicationIdentity)
	} else {
		fedCredsReady, err = r.updateFederatedCredentialsStatus(ctx, applicationIdentity, *fedCreds)
		if err != nil {
			logger.Error(err, "unable to update ApplicationIdentity status")
			return ctrl.Result{}, err
		}
	}
	if !fedCredsReady {
		return ctrl.Result{}, nil
	}

	// CHeck ServiceAccount status
	sa := &corev1.ServiceAccount{}
	err = r.Get(ctx, client.ObjectKey{
		Name:      applicationIdentity.Name,
		Namespace: applicationIdentity.Namespace,
	}, sa)
	if client.IgnoreNotFound(err) != nil {
		logger.Error(err, "unable to fetch ServiceAccount")
		return ctrl.Result{}, err
	} else if errors.IsNotFound(err) {
		err = r.createServiceAccount(ctx, applicationIdentity)
		if err != nil {
			logger.Error(err, "unable to create ServiceAccount")
			return ctrl.Result{}, err
		}
	} else {
		err = r.updateServiceAccount(ctx, applicationIdentity, sa)
		if err != nil {
			logger.Error(err, "unable to update ServiceAccount")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func getMetav1ConditionFromAzureCondition(conditionType applicationv1alpha1.ConditionType, azureCondition conditions.Condition, generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               string(conditionType),
		Status:             azureCondition.Status,
		LastTransitionTime: metav1.Now(),
		Reason:             azureCondition.Reason,
		Message:            azureCondition.Message,
		ObservedGeneration: generation,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationIdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&applicationv1alpha1.ApplicationIdentity{}).
		Owns(&managedidentity.UserAssignedIdentity{}).
		Named("applicationidentity").
		Complete(r)
}

func getReadyConditionFromStatus(status []conditions.Condition) conditions.Condition {
	for _, condition := range status {
		if condition.Type == "Ready" {
			return condition
		}
	}
	return conditions.Condition{
		Type:    "Ready",
		Status:  "False",
		Reason:  "NoStatus",
		Message: "No status available from the underlying resource",
	}
}
