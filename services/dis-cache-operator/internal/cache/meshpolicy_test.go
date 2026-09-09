package cache

import (
	"slices"
	"testing"
)

func TestBuildMeshPoliciesServers(t *testing.T) {
	t.Parallel()

	policies := BuildMeshPolicies(newTestCache(nil))

	if policies.ClientServer.Name != "app-one-cache-cache-client" || policies.BusServer.Name != "app-one-cache-cache-bus" {
		t.Fatalf("unexpected server names: %s, %s", policies.ClientServer.Name, policies.BusServer.Name)
	}
	if policies.ClientServer.Spec.Port.IntValue() != 6379 || policies.BusServer.Spec.Port.IntValue() != 16379 {
		t.Errorf("ports: want 6379 and 16379, got %v and %v", policies.ClientServer.Spec.Port, policies.BusServer.Spec.Port)
	}
	for _, server := range []string{policies.ClientServer.Spec.ProxyProtocol, policies.BusServer.Spec.ProxyProtocol} {
		if server != "opaque" {
			t.Errorf("proxy protocol: want opaque, got %q", server)
		}
	}
	if got := policies.ClientServer.Spec.PodSelector.MatchLabels[valkeyClusterLabel]; got != "app-one-cache" {
		t.Errorf("pod selector: want the Valkey pods of app-one-cache, got %q", got)
	}
	if policies.ClientServer.Namespace != "team-a" || policies.ClientServer.Labels[ManagedByLabel] != ManagedByValue {
		t.Errorf("namespace/labels: got %s %v", policies.ClientServer.Namespace, policies.ClientServer.Labels)
	}
}

func TestBuildMeshPoliciesAuthentication(t *testing.T) {
	t.Parallel()

	policies := BuildMeshPolicies(newTestCache(nil))

	if policies.Authentication.Name != "app-one-cache-cache-clients" {
		t.Errorf("authentication name: got %q", policies.Authentication.Name)
	}
	want := []string{
		"*.team-a.serviceaccount.identity.linkerd.cluster.local",
		"valkey-operator.valkey-operator-system.serviceaccount.identity.linkerd.cluster.local",
	}
	if !slices.Equal(policies.Authentication.Spec.Identities, want) {
		t.Errorf("identities: want %v, got %v", want, policies.Authentication.Spec.Identities)
	}
}

func TestBuildMeshPoliciesAuthorizations(t *testing.T) {
	t.Parallel()

	policies := BuildMeshPolicies(newTestCache(nil))

	if policies.ClientPolicy.Name != policies.ClientServer.Name || policies.BusPolicy.Name != policies.BusServer.Name {
		t.Errorf("policy names must match their servers: %s/%s, %s/%s",
			policies.ClientPolicy.Name, policies.ClientServer.Name, policies.BusPolicy.Name, policies.BusServer.Name)
	}
	target := policies.BusPolicy.Spec.TargetRef
	if string(target.Group) != "policy.linkerd.io" || string(target.Kind) != "Server" || string(target.Name) != "app-one-cache-cache-bus" {
		t.Errorf("bus target ref: got %+v", target)
	}
	refs := policies.ClientPolicy.Spec.RequiredAuthenticationRefs
	if len(refs) != 1 || string(refs[0].Kind) != "MeshTLSAuthentication" || string(refs[0].Name) != policies.Authentication.Name {
		t.Errorf("required authentication refs: got %+v", refs)
	}
}
