package sweepcache

import (
	"context"
	"testing"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
)

func TestSplitSeparatesUnchangedObjects(t *testing.T) {
	t.Parallel()

	resources := []flux.Resource{
		{Kind: "Kustomization", Namespace: "team-a", Name: "app-one", ContentHash: "aaa"},
		{Kind: "Kustomization", Namespace: "team-a", Name: "app-two", ContentHash: "bbb"},
		{Kind: "HelmRelease", Namespace: "team-b", Name: "app-three", ContentHash: "ccc"},
	}
	known := map[string]string{
		Key("v1", &resources[0]): "aaa", // same hash: unchanged
		Key("v1", &resources[1]): "old", // different hash: changed
		// app-three has no entry: changed
	}

	changed, unchanged := Split("v1", resources, known)

	if len(unchanged) != 1 || unchanged[0].Name != "app-one" {
		t.Errorf("unchanged: want [app-one], got %v", names(unchanged))
	}
	if len(changed) != 2 || changed[0].Name != "app-two" || changed[1].Name != "app-three" {
		t.Errorf("changed: want [app-two app-three], got %v", names(changed))
	}
}

func TestSplitTreatsAnotherBuildAsChanged(t *testing.T) {
	t.Parallel()

	resources := []flux.Resource{{Kind: "Kustomization", Namespace: "team-a", Name: "app-one", ContentHash: "aaa"}}
	known := map[string]string{Key("v1", &resources[0]): "aaa"}

	changed, unchanged := Split("v2", resources, known)
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
