package config

import (
	"strings"
	"testing"
)

const testTenantUUID = "00000000-0000-0000-0000-000000000000"

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

	cfg, err := NewOperatorConfig(
		"sub",
		"rg",
		testTenantUUID,
		"westeurope",
		"dev",
		"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/snet",
		"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/vpn-exit",
	)
	if err != nil {
		t.Fatalf("expected config to be valid, got error: %v", err)
	}
	if len(cfg.AKSSubnetIDs) != 2 {
		t.Fatalf("expected config subnet list to include optional vpn exit subnet, got %v", cfg.AKSSubnetIDs)
	}

	cfg, err = NewOperatorConfig(
		"sub",
		"rg",
		testTenantUUID,
		"westeurope",
		"dev",
		"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/snet,/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/vpn-exit",
		"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/vpn-exit",
	)
	if err != nil {
		t.Fatalf("expected duplicate vpn exit subnet to be accepted, got error: %v", err)
	}
	if len(cfg.AKSSubnetIDs) != 2 {
		t.Fatalf("expected duplicate vpn exit subnet not to be appended twice, got %v", cfg.AKSSubnetIDs)
	}

	if _, err := NewOperatorConfig("", "rg", "tenant", "westeurope", "dev", "x", ""); err == nil {
		t.Fatalf("expected error for missing required fields")
	}

	if _, err := NewOperatorConfig("sub", "rg", "tenant", "westeurope", "dev", "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/snet", ""); err == nil {
		t.Fatalf("expected error for invalid tenant UUID")
	}

	if _, err := NewOperatorConfig(
		"sub",
		"rg",
		testTenantUUID,
		"westeurope",
		"dev",
		"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/snet",
		"not-an-arm-id",
	); err == nil {
		t.Fatalf("expected error for invalid vpn exit node subnet id")
	}
}
