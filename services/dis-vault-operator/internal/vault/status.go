package vault

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
)

// ASOReadyCondition is a minimal condition projection input for tests-first wiring.
type ASOReadyCondition struct {
	Status  metav1.ConditionStatus
	Reason  string
	Message string
	Found   bool
}

// ProjectReadinessStatus projects ASO readiness into Vault status conditions.
func ProjectReadinessStatus(v *vaultv1alpha1.Vault, vaultReady ASOReadyCondition, roleAssignmentReady ASOReadyCondition) bool {
	if v == nil {
		return false
	}

	changed := false

	vaultCondition := toCondition(
		string(vaultv1alpha1.ConditionVaultReady),
		vaultReady,
		"VaultNotReady",
		"waiting for ASO Key Vault readiness",
	)
	if meta.SetStatusCondition(&v.Status.Conditions, vaultCondition) {
		changed = true
	}

	roleCondition := toCondition(
		string(vaultv1alpha1.ConditionRoleAssignmentReady),
		roleAssignmentReady,
		"RoleAssignmentNotReady",
		"waiting for ASO RoleAssignment readiness",
	)
	if meta.SetStatusCondition(&v.Status.Conditions, roleCondition) {
		changed = true
	}

	overall := aggregateReady(vaultReady, roleAssignmentReady)
	if meta.SetStatusCondition(&v.Status.Conditions, overall) {
		changed = true
	}

	if v.Status.ObservedGeneration != v.Generation {
		v.Status.ObservedGeneration = v.Generation
		changed = true
	}

	return changed
}

func toCondition(condType string, input ASOReadyCondition, notReadyReason, notReadyMessage string) metav1.Condition {
	if !input.Found {
		return metav1.Condition{
			Type:               condType,
			Status:             metav1.ConditionUnknown,
			Reason:             "NotFound",
			Message:            "dependent resource not found",
			ObservedGeneration: 0,
		}
	}

	reason := input.Reason
	if reason == "" {
		if input.Status == metav1.ConditionTrue {
			reason = "Ready"
		} else {
			reason = notReadyReason
		}
	}

	message := input.Message
	if message == "" {
		if input.Status == metav1.ConditionTrue {
			message = "dependency is ready"
		} else {
			message = notReadyMessage
		}
	}

	return metav1.Condition{
		Type:    condType,
		Status:  input.Status,
		Reason:  reason,
		Message: message,
	}
}

func aggregateReady(vaultReady ASOReadyCondition, roleAssignmentReady ASOReadyCondition) metav1.Condition {
	switch {
	case !vaultReady.Found || !roleAssignmentReady.Found:
		return metav1.Condition{
			Type:    string(vaultv1alpha1.ConditionReady),
			Status:  metav1.ConditionUnknown,
			Reason:  "DependenciesPending",
			Message: "waiting for dependent resources",
		}
	case vaultReady.Status == metav1.ConditionTrue && roleAssignmentReady.Status == metav1.ConditionTrue:
		return metav1.Condition{
			Type:    string(vaultv1alpha1.ConditionReady),
			Status:  metav1.ConditionTrue,
			Reason:  "Ready",
			Message: "all dependencies are ready",
		}
	case vaultReady.Status == metav1.ConditionFalse || roleAssignmentReady.Status == metav1.ConditionFalse:
		return metav1.Condition{
			Type:    string(vaultv1alpha1.ConditionReady),
			Status:  metav1.ConditionFalse,
			Reason:  "DependencyNotReady",
			Message: "one or more dependencies are not ready",
		}
	default:
		return metav1.Condition{
			Type:    string(vaultv1alpha1.ConditionReady),
			Status:  metav1.ConditionUnknown,
			Reason:  "DependencyUnknown",
			Message: "dependency readiness is unknown",
		}
	}
}
