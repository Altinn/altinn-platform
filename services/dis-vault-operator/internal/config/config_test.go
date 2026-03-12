package config

import (
	"strings"
	"testing"
)

func TestParseSubnetIDs(t *testing.T) {
	t.Parallel()

	t.Run("parses valid comma-separated subnet IDs", func(t *testing.T) {
		t.Parallel()

		ids, err := ParseSubnetIDs(strings.Join([]string{
			"/subscriptions/sub-a/resourceGroups/rg-a/providers/Microsoft.Network/virtualNetworks/vnet-a/subnets/snet-a",
			"/subscriptions/sub-a/resourceGroups/rg-a/providers/Microsoft.Network/virtualNetworks/vnet-a/subnets/snet-b",
		}, ","))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ids) != 2 {
			t.Fatalf("expected 2 subnet IDs, got %d", len(ids))
		}
	})

	t.Run("rejects empty list", func(t *testing.T) {
		t.Parallel()

		if _, err := ParseSubnetIDs(" , "); err == nil {
			t.Fatalf("expected error for empty subnet IDs")
		}
	})

	t.Run("rejects malformed subnet ID", func(t *testing.T) {
		t.Parallel()

		if _, err := ParseSubnetIDs("/subscriptions/sub-a/not-a-valid-id"); err == nil {
			t.Fatalf("expected error for malformed subnet ID")
		}
	})
}

func TestNewOperatorConfig(t *testing.T) {
	t.Parallel()

	_, err := NewOperatorConfig(
		"sub",
		"rg",
		"tenant",
		"westeurope",
		"dev",
		"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/snet",
	)
	if err != nil {
		t.Fatalf("expected config to be valid, got error: %v", err)
	}

	if _, err := NewOperatorConfig("", "rg", "tenant", "westeurope", "dev", "x"); err == nil {
		t.Fatalf("expected error for missing required fields")
	}
}
