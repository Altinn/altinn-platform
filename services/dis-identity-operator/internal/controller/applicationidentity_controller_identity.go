package controller

import (
	"context"

	managedidentity "github.com/Azure/azure-service-operator/v2/api/managedidentity/v1api20230131"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	applicationv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-identity-operator/internal/utils"
)

func (r *ApplicationIdentityReconciler) removeUserAssignedIdentity(ctx context.Context, applicationIdentity *applicationv1alpha1.ApplicationIdentity) (bool, error) {
	logger := logf.FromContext(ctx)
	uaID := &managedidentity.UserAssignedIdentity{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      applicationIdentity.Name,
		Namespace: applicationIdentity.Namespace,
	}, uaID)
	if err != nil {
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	if uaID != nil && uaID.GetDeletionTimestamp() == nil && utils.IsOwnedBy(uaID, applicationIdentity) {
		if err := r.Delete(ctx, uaID); err != nil {
			logger.Error(err, "unable to delete UserAssignedIdentity")
			return false, err
		}
	}
	return false, nil
}

func (r *ApplicationIdentityReconciler) createNewUserAssignedIdentity(ctx context.Context, applicationIdentity *applicationv1alpha1.ApplicationIdentity) error {
	logger := logf.FromContext(ctx)
	// Create a new UserAssignedIdentity object
	uaID := applicationIdentity.GenerateUserAssignedIdentity(r.Config.TargetResourceGroup)
	err := controllerutil.SetControllerReference(applicationIdentity, uaID, r.Scheme)
	if err != nil {
		logger.Error(err, "unable to set controller reference for UserAssignedIdentity")
		return err
	}
	// Create the UserAssignedIdentity
	if err := r.Create(ctx, uaID); err != nil {
		logger.Error(err, "unable to create UserAssignedIdentity")
		return err
	}
	return nil
}

func (r *ApplicationIdentityReconciler) updateUserAssignedIdentityStatus(ctx context.Context, applicationIdentity *applicationv1alpha1.ApplicationIdentity, uaID *managedidentity.UserAssignedIdentity) (bool, error) {
	logger := logf.FromContext(ctx)
	// Update the status of the ApplicationIdentity from the UserAssignedIdentity status
	if applicationIdentity.OutdatedUserAssignedIdentity(uaID) {
		origUaID := uaID.DeepCopy()
		uaIDPatch := client.MergeFrom(origUaID)
		uaID.Spec.Tags = applicationIdentity.GetUserAssignedIdentityTags()
		if err := r.Patch(ctx, uaID, uaIDPatch); err != nil {
			apiErr := err.(errors.APIStatus)
			logger.Error(err, "unable to update UserAssignedIdentity", "error", apiErr.Status().Reason)
			return false, err
		}
		return false, nil
	}
	readyCondition := getReadyConditionFromStatus(uaID.Status.Conditions)
	ready := false
	orig := applicationIdentity.DeepCopy()
	patch := client.MergeFrom(orig)
	if readyCondition.Status == "True" {
		applicationIdentity.Status.PrincipalID = uaID.Status.PrincipalId
		applicationIdentity.Status.ClientID = uaID.Status.ClientId
		applicationIdentity.Status.ManagedIdentityName = utils.ToPointer(uaID.Spec.AzureName)
		ready = true
	}
	applicationIdentity.ReplaceCondition(applicationv1alpha1.ConditionUserAssignedIdentityType, getMetav1ConditionFromAzureCondition(applicationv1alpha1.ConditionUserAssignedIdentityType, readyCondition, applicationIdentity.Generation))
	if err := r.Status().Patch(ctx, applicationIdentity, patch); err != nil {
		apiErr := err.(errors.APIStatus)
		logger.Error(err, "unable to update ApplicationIdentity status", "error", apiErr.Status().Reason)
		return false, err
	}
	return ready, nil
}
