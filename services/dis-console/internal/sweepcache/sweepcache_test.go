package sweepcache

import (
	"context"
	"testing"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
)

func testClient(prefix string) *Client { return &Client{prefix: prefix} }

func TestSplitSeparatesUnchangedObjects(t *testing.T) {
	t.Parallel()

	resources := []flux.Resource{
		{Kind: "Kustomization", Namespace: "team-a", Name: "app-one", ContentHash: "aaa"},
		{Kind: "Kustomization", Namespace: "team-a", Name: "app-two", ContentHash: "bbb"},
		{Kind: "HelmRelease", Namespace: "team-b", Name: "app-three", ContentHash: "ccc"},
	}
	c := testClient("v1")
	known := map[string]string{
		key("v1", &resources[0]): fingerprint(&resources[0]), // same: unchanged
		key("v1", &resources[1]): "old|",                     // different hash: changed
		// app-three has no entry: changed
	}

	changed, unchanged := c.Split(resources, known)

	if len(unchanged) != 1 || unchanged[0].Name != "app-one" {
		t.Errorf("unchanged: want [app-one], got %v", names(unchanged))
	}
	if len(changed) != 2 || changed[0].Name != "app-two" || changed[1].Name != "app-three" {
		t.Errorf("changed: want [app-two app-three], got %v", names(changed))
	}
}

func TestSplitSeesAChangedApplier(t *testing.T) {
	t.Parallel()

	before := flux.Resource{Kind: "HelmRelease", Namespace: "team-a", Name: "app-one", ContentHash: "aaa",
		AppliedBy: &flux.AppliedBy{Name: "release-one", Namespace: "team-a"}}
	after := before
	after.AppliedBy = &flux.AppliedBy{Name: "release-two", Namespace: "team-a"}
	c := testClient("v1")
	known := map[string]string{key("v1", &before): fingerprint(&before)}

	changed, unchanged := c.Split([]flux.Resource{after}, known)
	if len(changed) != 1 || len(unchanged) != 0 {
		t.Errorf("a new applier with the same hash must count as changed, got changed=%d unchanged=%d", len(changed), len(unchanged))
	}
}

func TestSplitTreatsAnotherBuildAsChanged(t *testing.T) {
	t.Parallel()

	resources := []flux.Resource{{Kind: "Kustomization", Namespace: "team-a", Name: "app-one", ContentHash: "aaa"}}
	known := map[string]string{key("v1", &resources[0]): fingerprint(&resources[0])}

	changed, unchanged := testClient("v2").Split(resources, known)
	if len(changed) != 1 || len(unchanged) != 0 {
		t.Errorf("a new prefix must miss every entry, got changed=%d unchanged=%d", len(changed), len(unchanged))
	}
}

func TestNilClientIsANoop(t *testing.T) {
	t.Parallel()

	var c *Client
	resources := []flux.Resource{{Kind: "Kustomization", Namespace: "team-a", Name: "app-one", ContentHash: "aaa"}}

	hashes, err := c.Hashes(context.Background(), resources)
	if err != nil || len(hashes) != 0 {
		t.Errorf("nil client Hashes: want empty, got %v, %v", hashes, err)
	}
	if err := c.Remember(context.Background(), resources); err != nil {
		t.Errorf("nil client Remember: %v", err)
	}
	changed, unchanged := c.Split(resources, map[string]string{key("v1", &resources[0]): fingerprint(&resources[0])})
	if len(changed) != 1 || len(unchanged) != 0 {
		t.Errorf("nil client Split: want everything changed, got changed=%d unchanged=%d", len(changed), len(unchanged))
	}
	c.Close()
}

func TestNewWithoutAddressIsDisabled(t *testing.T) {
	t.Parallel()

	c, err := New(context.Background(), Config{}, "v1")
	if err != nil || c != nil {
		t.Errorf("empty address: want nil client and no error, got %v, %v", c, err)
	}
}

func names(resources []flux.Resource) []string {
	out := make([]string, len(resources))
	for i := range resources {
		out[i] = resources[i].Name
	}

	return out
}
