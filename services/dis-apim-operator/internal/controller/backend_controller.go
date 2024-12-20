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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apimv1alpha1 "github.com/Altinn/altinn-platform/services/dis-apim-operator/api/v1alpha1"
)

const BACKEND_FINALIZER = "backend.apim.dis.altinn.cloud/finalizer"

type newApimClient func(config *azure.ApimClientConfig) (*azure.APIMClient, error)

// BackendReconciler reconciles a Backend object
type BackendReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	ApimClientConfig *azure.ApimClientConfig
	NewClient        newApimClient
	apimClient       *azure.APIMClient
}

// +kubebuilder:rbac:groups=apim.dis.altinn.cloud,resources=backends,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apim.dis.altinn.cloud,resources=backends/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apim.dis.altinn.cloud,resources=backends/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Backend object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *BackendReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var backend apimv1alpha1.Backend
	if err := r.Get(ctx, req.NamespacedName, &backend); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "unable to fetch Backend")
			return ctrl.Result{}, err
		}
		// Object not found, return and don't requeue
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&backend, BACKEND_FINALIZER) {
		controllerutil.AddFinalizer(&backend, BACKEND_FINALIZER)
		if err := r.Update(ctx, &backend); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}
	c, err := r.NewClient(r.ApimClientConfig)
	if err != nil {
		logger.Error(err, "Failed to create APIM client")
		return ctrl.Result{}, err
	}
	r.apimClient = c
	if backend.DeletionTimestamp != nil {
		return ctrl.Result{}, r.handleDeletion(ctx, &backend)
	}
	azBackend, err := r.apimClient.GetBackend(ctx, backend.GetAzureResourceName(), nil)
	if err != nil {
		if azure.IsNotFoundError(err) {
			if err := r.handleCreateUpdate(ctx, &backend); err != nil {
				logger.Error(err, "Failed to create backend")
				return ctrl.Result{}, err
			}
			logger.Info("Backend created")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
		logger.Error(err, "Failed to get backend")
		return ctrl.Result{}, err
	}
	if !backend.MatchesActualState(&azBackend) {
		logger.Info("Backend does not match actual state, updating")
		err := r.handleCreateUpdate(ctx, &backend)
		if err != nil {
			logger.Error(err, "Failed to update backend")
			return ctrl.Result{}, err
		}
		logger.Info("Backend updated")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}
	logger.Info("Backend matches actual state")
	if backend.Status.ProvisioningState != apimv1alpha1.BackendProvisioningStateSucceeded || backend.Status.BackendID != *azBackend.ID {
		backend.Status.ProvisioningState = apimv1alpha1.BackendProvisioningStateSucceeded
		backend.Status.BackendID = *azBackend.ID
		if err := r.Status().Update(ctx, &backend); err != nil {
			logger.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackendReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apimv1alpha1.Backend{}).
		Named("backend").
		Complete(r)
}

func (r *BackendReconciler) handleCreateUpdate(ctx context.Context, backend *apimv1alpha1.Backend) error {
	res, err := r.apimClient.CreateUpdateBackend(ctx, backend.GetAzureResourceName(), backend.ToAzureBackend(), nil)
	if err != nil {
		backend.Status.ProvisioningState = apimv1alpha1.BackendProvisioningStateFailed
		backend.Status.LastProvisioningError = fmt.Sprintf("err when creating backend: %v", err)
		if errUpdate := r.Status().Update(ctx, backend); errUpdate != nil {
			return fmt.Errorf("failed to update status to failed: %v", errUpdate)
		}
		return err
	}
	backend.Status.BackendID = *res.ID
	backend.Status.ProvisioningState = apimv1alpha1.BackendProvisioningStateSucceeded
	if errUpdate := r.Status().Update(ctx, backend); errUpdate != nil {
		return fmt.Errorf("failed to update status to succeeded: %v", errUpdate)
	}
	return nil
}

func (r *BackendReconciler) handleDeletion(ctx context.Context, backend *apimv1alpha1.Backend) error {
	logger := log.FromContext(ctx)
	azureBackend, err := r.apimClient.GetBackend(ctx, backend.GetAzureResourceName(), nil)
	if err != nil {
		if azure.IsNotFoundError(err) {
			controllerutil.RemoveFinalizer(backend, BACKEND_FINALIZER)
			if err := r.Update(ctx, backend); err != nil {
				logger.Error(err, "Failed to remove finalizer")
				return err
			}
			return nil
		}
		logger.Error(err, "Failed to get backend for deletion")
		return err
	}
	resp, err := r.apimClient.DeleteBackend(ctx, backend.GetAzureResourceName(), *azureBackend.ETag, nil)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to delete backend. backend: %#v", azureBackend))
		return err
	}
	logger.Info("Backend deleted", "response", resp)
	controllerutil.RemoveFinalizer(backend, BACKEND_FINALIZER)
	if err := r.Update(ctx, backend); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return err
	}
	return nil
}
