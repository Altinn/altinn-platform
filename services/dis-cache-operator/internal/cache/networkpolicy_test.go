package cache

import (
	"testing"

	netv1 "k8s.io/api/networking/v1"
)

func TestBuildNetworkPolicy(t *testing.T) {
	t.Parallel()

	policy := BuildNetworkPolicy(newTestCache(nil))

	if policy.Name != "app-one-cache-cache" || policy.Namespace != "team-a" {
		t.Fatalf("unexpected name/namespace: %s/%s", policy.Namespace, policy.Name)
	}
	if policy.Labels[CacheNameLabel] != "app-one-cache" {
		t.Errorf("missing cache name label, got %v", policy.Labels)
	}

	if got := policy.Spec.PodSelector.MatchLabels[valkeyClusterLabel]; got != "app-one-cache" {
		t.Errorf("pod selector: want valkey pods of app-one-cache, got %q", got)
	}
	if len(policy.Spec.PolicyTypes) != 1 || policy.Spec.PolicyTypes[0] != netv1.PolicyTypeIngress {
		t.Errorf("policy types: want [Ingress], got %v", policy.Spec.PolicyTypes)
	}
	if len(policy.Spec.Ingress) != 3 {
		t.Fatalf("ingress rules: want 3, got %d", len(policy.Spec.Ingress))
	}

	sameNamespace := policy.Spec.Ingress[0]
	if sameNamespace.From[0].PodSelector == nil || len(sameNamespace.From[0].PodSelector.MatchLabels) != 0 {
		t.Errorf("rule 0: want an empty pod selector (all pods in the namespace), got %+v", sameNamespace.From[0])
	}
	if len(sameNamespace.Ports) != 1 || sameNamespace.Ports[0].Port.IntValue() != 6379 {
		t.Errorf("rule 0 ports: want [6379], got %+v", sameNamespace.Ports)
	}

	valkeyToValkey := policy.Spec.Ingress[1]
	if got := valkeyToValkey.From[0].PodSelector.MatchLabels[valkeyClusterLabel]; got != "app-one-cache" {
		t.Errorf("rule 1: want the valkey pods themselves, got %q", got)
	}
	if len(valkeyToValkey.Ports) != 2 || valkeyToValkey.Ports[1].Port.IntValue() != 16379 {
		t.Errorf("rule 1 ports: want [6379 16379], got %+v", valkeyToValkey.Ports)
	}

	operator := policy.Spec.Ingress[2]
	if operator.From[0].NamespaceSelector == nil {
		t.Fatal("rule 2: want a namespace selector for the operator namespace")
	}
	if got := operator.From[0].NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"]; got != operatorNamespace {
		t.Errorf("rule 2: want namespace %s, got %q", operatorNamespace, got)
	}
	if operator.From[0].PodSelector == nil {
		t.Fatal("rule 2: want a pod selector for the operator pods")
	}
	if got := operator.From[0].PodSelector.MatchLabels["app.kubernetes.io/name"]; got != operatorPodLabelValue {
		t.Errorf("rule 2: want operator pods %s, got %q", operatorPodLabelValue, got)
	}
	if got := operator.From[0].PodSelector.MatchLabels["control-plane"]; got != "controller-manager" {
		t.Errorf("rule 2: want control-plane controller-manager, got %q", got)
	}
	if len(operator.Ports) != 1 || operator.Ports[0].Port.IntValue() != 6379 {
		t.Errorf("rule 2 ports: want [6379], got %+v", operator.Ports)
	}
}
