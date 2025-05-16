package controller

import (
	"context"

	managedidentity "github.com/Azure/azure-service-operator/v2/api/managedidentity/v1api20230131"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	applicationv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
)

func (r *ApplicationIdentityReconciler) createFederation(ctx context.Context, applicationIdentity *applicationv1alpha1.ApplicationIdentity) error {
	logger := logf.FromContext(ctx)
	// Create a new FederatedIdentityCredential object
	federatedCredential := applicationIdentity.GenerateFederatedCredentials(r.Config.IssuerURL)
	err := controllerutil.SetControllerReference(applicationIdentity, federatedCredential, r.Scheme)
	if err != nil {
		logger.Error(err, "unable to set controller reference for FederatedIdentityCredential")
		return err
	}
	// Create the FederatedIdentityCredential
	if err := r.Create(ctx, federatedCredential); err != nil {
		logger.Error(err, "unable to create FederatedIdentityCredential")
		return err
	}
	return nil
}

func (r *ApplicationIdentityReconciler) updateFederatedCredentialsStatus(ctx context.Context, applicationIdentity *applicationv1alpha1.ApplicationIdentity, credential *managedidentity.FederatedIdentityCredential) (bool, error) {
	logger := logf.FromContext(ctx)
	// Update the status of the ApplicationIdentity from the UserAssignedIdentity status
	ready := false
	orig := applicationIdentity.DeepCopy()
	patch := client.MergeFrom(orig)
	if applicationIdentity.OutdatedFederatedCredentials(credential) {
		credOrig := applicationIdentity.DeepCopy()
		credPatch := client.MergeFrom(credOrig)
		credential.Spec.Audiences = applicationIdentity.Spec.AzureAudiences
		if err := r.Patch(ctx, credential, credPatch); err != nil {
			logger.Error(err, "unable to update FederatedIdentityCredential")
			return false, err
		}
		return false, nil
	}

	// Check if the FederatedIdentityCredential is ready
	readyCondition := getReadyConditionFromStatus(credential.Status.Conditions)
	if readyCondition.Status == "True" {
		applicationIdentity.Status.AzureAudiences = credential.Status.Audiences
		ready = true
	}
	applicationIdentity.ReplaceCondition(applicationv1alpha1.ConditionFederatedIdentityType, getMetav1ConditionFromAzureCondition(applicationv1alpha1.ConditionFederatedIdentityType, readyCondition, applicationIdentity.Generation))
	if err := r.Status().Patch(ctx, applicationIdentity, patch); err != nil {
		apiErr := err.(errors.APIStatus)
		logger.Error(err, "unable to update ApplicationIdentity status", "error", apiErr.Status().Reason)
		return false, err
	}
	return ready, nil
}
