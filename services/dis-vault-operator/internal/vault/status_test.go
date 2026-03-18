package vault

import (
	"testing"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProjectReadinessStatus(t *testing.T) {
	t.Parallel()

	v := &vaultv1alpha1.Vault{}
	v.Generation = 5

	updated := ProjectReadinessStatus(v,
		ASOReadyCondition{Found: true, Status: metav1.ConditionTrue, Reason: "Ready"},
		ASOReadyCondition{Found: true, Status: metav1.ConditionTrue, Reason: "Ready"},
	)
	if !updated {
		t.Fatalf("expected status projection to update status")
	}

	if v.Status.ObservedGeneration != v.Generation {
		t.Fatalf("expected observedGeneration=%d, got %d", v.Generation, v.Status.ObservedGeneration)
	}

	ready := findCondition(v.Status.Conditions, string(vaultv1alpha1.ConditionReady))
	if ready == nil {
		t.Fatalf("expected Ready condition to be projected")
	}
	if ready.Status != metav1.ConditionTrue {
		t.Fatalf("expected Ready=True, got %s", ready.Status)
	}
}

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
