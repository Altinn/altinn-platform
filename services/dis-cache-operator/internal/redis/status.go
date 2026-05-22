package redis

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	redisv1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
	asoconditions "github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
)

// ASOReadyCondition is a projected ASO Ready condition used by the controller.
type ASOReadyCondition struct {
	Status  metav1.ConditionStatus
	Reason  string
	Message string
	Found   bool
}

// FromASOConditions projects the Ready condition from a slice of ASO conditions.
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

// NewCondition builds a standard metav1.Condition for the Redis status.
func NewCondition(
	conditionType redisv1alpha1.ConditionType,
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

// AggregateReadyCondition combines child conditions into a single Ready condition.
func AggregateReadyCondition(generation int64, conditions ...metav1.Condition) metav1.Condition {
	if len(conditions) == 0 {
		return NewCondition(
			redisv1alpha1.ConditionReady,
			generation,
			metav1.ConditionUnknown,
			"NoDependencies",
			"no dependency conditions present",
		)
	}

	hasFalse := false
	hasUnknown := false
	for _, cond := range conditions {
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
			redisv1alpha1.ConditionReady,
			generation,
			metav1.ConditionFalse,
			"DependencyNotReady",
			"one or more dependencies are not ready",
		)
	case hasUnknown:
		return NewCondition(
			redisv1alpha1.ConditionReady,
			generation,
			metav1.ConditionUnknown,
			"DependenciesPending",
			"waiting for dependent resources",
		)
	default:
		return NewCondition(
			redisv1alpha1.ConditionReady,
			generation,
			metav1.ConditionTrue,
			"Ready",
			"all dependencies are ready",
		)
	}
}
