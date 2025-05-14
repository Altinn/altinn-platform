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
		_ = r.patchReadyStatusCondition(ctx, applicationIdentity, metav1.Condition{
			Type:               string(v1alpha1.ConditionReady),
			Status:             "False",
			ObservedGeneration: applicationIdentity.Generation,
			LastTransitionTime: metav1.Now(),
			Reason:             "Failed to create ServiceAccount",
			Message:            "Unable to set controller reference for ServiceAccount",
		})
		return fmt.Errorf("unable to set controller reference for ServiceAccount: %w", err)
	}
	if err := r.Create(ctx, sa); err != nil {
		_ = r.patchReadyStatusCondition(ctx, applicationIdentity, metav1.Condition{
			Type:               string(v1alpha1.ConditionReady),
			Status:             "False",
			ObservedGeneration: applicationIdentity.Generation,
			LastTransitionTime: metav1.Now(),
			Reason:             "Failed to create ServiceAccount",
			Message:            "Unable to create ServiceAccount",
		})
		return fmt.Errorf("unable to create ServiceAccount: %w", err)
	}
	err := r.patchReadyStatusCondition(ctx, applicationIdentity, metav1.Condition{
		Type:               string(v1alpha1.ConditionReady),
		Status:             "True",
		ObservedGeneration: applicationIdentity.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             "Succeeded",
		Message:            "",
	})
	if err != nil {
		return fmt.Errorf("unable to update ApplicationIdentity status: %w", err)
	}
	return nil
}

func (r *ApplicationIdentityReconciler) updateServiceAccount(ctx context.Context, applicationIdentity *v1alpha1.ApplicationIdentity, serviceAccount *corev1.ServiceAccount) error {
	orig := serviceAccount.DeepCopy()
	patch := client.MergeFrom(orig)

	serviceAccount.Labels = applicationIdentity.Spec.Tags
	serviceAccount.Annotations = map[string]string{
		"serviceaccount.azure.com/azure-identity": *applicationIdentity.Status.ClientID,
	}
	if err := r.Patch(ctx, serviceAccount, patch); err != nil {
		_ = r.patchReadyStatusCondition(ctx, applicationIdentity, metav1.Condition{
			Type:               string(v1alpha1.ConditionReady),
			Status:             "False",
			ObservedGeneration: applicationIdentity.Generation,
			LastTransitionTime: metav1.Now(),
			Reason:             "Failed to update ServiceAccount",
			Message:            "Unable to update ServiceAccount",
		})
		return fmt.Errorf("unable to update ServiceAccount: %w", err)
	}
	err := r.patchReadyStatusCondition(ctx, applicationIdentity, metav1.Condition{
		Type:               string(v1alpha1.ConditionReady),
		Status:             "True",
		ObservedGeneration: applicationIdentity.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             "Succeeded",
		Message:            "",
	})
	if err != nil {
		return fmt.Errorf("unable to update ApplicationIdentity status: %w", err)
	}
	return nil
}

func (r *ApplicationIdentityReconciler) patchReadyStatusCondition(ctx context.Context, applicationIdentity *v1alpha1.ApplicationIdentity, condition metav1.Condition) error {
	orig := applicationIdentity.DeepCopy()
	patch := client.MergeFrom(orig)
	applicationIdentity.ReplaceCondition(v1alpha1.ConditionReady, condition)
	if err := r.Status().Patch(ctx, applicationIdentity, patch); err != nil {
		return fmt.Errorf("unable to update ApplicationIdentity status: %w", err)
	}
	return nil
}
