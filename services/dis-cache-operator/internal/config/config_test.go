package config

import (
	"strings"
	"testing"
)

const (
	testTenantUUID = "00000000-0000-0000-0000-000000000000"
	testSubnetID   = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/snet"
	testVNetID     = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet"
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

	cfg, err := NewOperatorConfig(
		"sub", "rg", testTenantUUID, "norwayeast", "dev",
		testSubnetID, testVNetID, "rg-dns",
	)
	if err != nil {
		t.Fatalf("expected config to be valid, got error: %v", err)
	}
	if cfg.PrimarySubnetID() != testSubnetID {
		t.Fatalf("expected primary subnet to match input, got %q", cfg.PrimarySubnetID())
	}
	if cfg.DNSZoneResourceGroup != "rg-dns" {
		t.Fatalf("expected DNS zone RG to be set, got %q", cfg.DNSZoneResourceGroup)
	}

	if _, err := NewOperatorConfig("", "rg", testTenantUUID, "norwayeast", "dev", testSubnetID, testVNetID, "rg-dns"); err == nil {
		t.Fatalf("expected error for missing required fields")
	}

	if _, err := NewOperatorConfig("sub", "rg", "not-a-uuid", "norwayeast", "dev", testSubnetID, testVNetID, "rg-dns"); err == nil {
		t.Fatalf("expected error for invalid tenant UUID")
	}

	if _, err := NewOperatorConfig("sub", "rg", testTenantUUID, "norwayeast", "dev", testSubnetID, "not-a-vnet-id", "rg-dns"); err == nil {
		t.Fatalf("expected error for invalid vnet id")
	}

	if _, err := NewOperatorConfig("sub", "rg", testTenantUUID, "norwayeast", "dev", "", testVNetID, "rg-dns"); err == nil {
		t.Fatalf("expected error for missing subnet ids")
	}
}
