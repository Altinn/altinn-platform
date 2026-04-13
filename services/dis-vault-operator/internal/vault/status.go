package vault

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	asoconditions "github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
)

// ASOReadyCondition is a projected ASO Ready condition used by the controller.
type ASOReadyCondition struct {
	Status  metav1.ConditionStatus
	Reason  string
	Message string
	Found   bool
}

func FromASOConditions(conditions []asoconditions.Condition) ASOReadyCondition {
	for _, cond := range conditions {
		if cond.Type != asoconditions.ConditionTypeReady {
			continue
		}
		return ASOReadyCondition{
			Found:   true,
			Status:  cond.Status,
			Reason:  cond.Reason,
			Message: cond.Message,
		}
	}

	return ASOReadyCondition{}
}

func NewCondition(
	conditionType vaultv1alpha1.ConditionType,
	generation int64,
	status metav1.ConditionStatus,
	reason, message string,
) metav1.Condition {
	return metav1.Condition{
		Type:               string(conditionType),
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	}
}

func AggregateReadyCondition(
	generation int64,
	identityReady,
	vaultReady,
	ownerRoleAssignmentReady,
	networkPolicyReady,
	externalSecretsReady metav1.Condition,
	extraRequired ...metav1.Condition,
) metav1.Condition {
	required := []metav1.Condition{
		identityReady,
		vaultReady,
		ownerRoleAssignmentReady,
		networkPolicyReady,
	}
	required = append(required, extraRequired...)
	if externalSecretsReady.Status != metav1.ConditionFalse || externalSecretsReady.Reason != "Disabled" {
		required = append(required, externalSecretsReady)
	}

	hasFalse := false
	hasUnknown := false
	for _, cond := range required {
		switch cond.Status {
		case metav1.ConditionFalse:
			hasFalse = true
		case metav1.ConditionUnknown:
			hasUnknown = true
		}
	}

	switch {
	case hasFalse:
		return NewCondition(
			vaultv1alpha1.ConditionReady,
			generation,
			metav1.ConditionFalse,
			"DependencyNotReady",
			"one or more dependencies are not ready",
		)
	case hasUnknown:
		return NewCondition(
			vaultv1alpha1.ConditionReady,
			generation,
			metav1.ConditionUnknown,
			"DependenciesPending",
			"waiting for dependent resources",
		)
	default:
		return NewCondition(
			vaultv1alpha1.ConditionReady,
			generation,
			metav1.ConditionTrue,
			"Ready",
			"all dependencies are ready",
		)
	}
}
