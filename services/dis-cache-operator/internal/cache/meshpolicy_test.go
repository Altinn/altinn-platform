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

func TestBuildMetricsPolicies(t *testing.T) {
	t.Parallel()

	cache := newTestCache(nil)
	metrics := BuildMetricsPolicies(cache, []string{"10.240.0.0/16", "fd00::/8"})

	if metrics.Server.Name != "app-one-cache-cache-metrics" || metrics.Server.Spec.Port.IntValue() != 9121 {
		t.Errorf("metrics server: want the exporter port 9121, got %s %v", metrics.Server.Name, metrics.Server.Spec.Port)
	}
	if metrics.Server.Spec.ProxyProtocol != "HTTP/1" {
		t.Errorf("metrics server: want HTTP/1, got %q", metrics.Server.Spec.ProxyProtocol)
	}
	if metrics.Server.Spec.PodSelector.MatchLabels["valkey.io/cluster"] != "app-one-cache" {
		t.Errorf("metrics server: want the Valkey pods selected, got %v", metrics.Server.Spec.PodSelector)
	}
	if metrics.Authentication.Name != "app-one-cache-cache-scrapers" || len(metrics.Authentication.Spec.Networks) != 2 {
		t.Fatalf("scraper authentication: want two networks, got %s %+v", metrics.Authentication.Name, metrics.Authentication.Spec.Networks)
	}
	if metrics.Authentication.Spec.Networks[0].Cidr != "10.240.0.0/16" || metrics.Authentication.Spec.Networks[1].Cidr != "fd00::/8" {
		t.Errorf("scraper authentication: want the given networks in order, got %+v", metrics.Authentication.Spec.Networks)
	}
	if metrics.Policy.Name != metrics.Server.Name || string(metrics.Policy.Spec.TargetRef.Name) != metrics.Server.Name {
		t.Errorf("metrics policy: want it bound to the metrics server, got %s -> %s", metrics.Policy.Name, metrics.Policy.Spec.TargetRef.Name)
	}
	ref := metrics.Policy.Spec.RequiredAuthenticationRefs
	if len(ref) != 1 || string(ref[0].Kind) != "NetworkAuthentication" || string(ref[0].Name) != metrics.Authentication.Name {
		t.Errorf("metrics policy: want the scraper NetworkAuthentication required, got %+v", ref)
	}
	for _, obj := range []interface{ GetLabels() map[string]string }{metrics.Server, metrics.Authentication, metrics.Policy} {
		if obj.GetLabels()[CacheNameLabel] != cache.Name {
			t.Errorf("metrics object without the cache label: %v", obj.GetLabels())
		}
	}
}
