package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
)

func (r *ApplicationIdentityReconciler) createServiceAccount(ctx context.Context, applicationIdentity *v1alpha1.ApplicationIdentity) error {
	if applicationIdentity.Status.ClientID == nil {
		return fmt.Errorf("applicationIdentity.Status.ClientID is nil")
	}
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      applicationIdentity.Name,
			Namespace: applicationIdentity.Namespace,
			Labels:    applicationIdentity.Spec.Tags,
			Annotations: map[string]string{
				"serviceaccount.azure.com/azure-identity": *applicationIdentity.Status.ClientID,
			},
		},
		Secrets:                      nil,
		ImagePullSecrets:             nil,
		AutomountServiceAccountToken: nil,
	}
	if err := controllerutil.SetControllerReference(applicationIdentity, sa, r.Scheme); err != nil {
		return fmt.Errorf("unable to set controller reference for ServiceAccount: %w", err)
	}
	if err := r.Create(ctx, sa); err != nil {
		return fmt.Errorf("unable to create ServiceAccount: %w", err)
	}
	return nil
}

func (r *ApplicationIdentityReconciler) updateServiceAccount(ctx context.Context, applicationIdentity *v1alpha1.ApplicationIdentity, serviceAccount *corev1.ServiceAccount) error {
	orig := serviceAccount.DeepCopy()
	patch := client.MergeFrom(orig)
	if applicationIdentity.OutdatedServiceAccount(serviceAccount) {
		serviceAccount.Labels = applicationIdentity.Spec.Tags
		serviceAccount.Annotations = map[string]string{
			"serviceaccount.azure.com/azure-identity": *applicationIdentity.Status.ClientID,
		}
		if err := r.Patch(ctx, serviceAccount, patch); err != nil {
			return fmt.Errorf("unable to update ServiceAccount: %w", err)
		}
		return nil
	}
	return nil
}
