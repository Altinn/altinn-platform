package network

import (
	"context"
	"errors"
	"testing"

	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/test/azfakes"
)

// fetchFakeVNetLocation runs FetchVNetLocation against a fake VNet in the given location.
func fetchFakeVNetLocation(t *testing.T, location string) (string, error) {
	t.Helper()

	env := azfakes.NewNetworkEnv(nil, azfakes.VirtualNetworksServerWithLocation(location))
	cfg := &config.OperatorConfig{
		SubscriptionId: "00000000-0000-0000-0000-000000000000",
		ResourceGroup:  "rg-fake",
		DBVNetName:     "vnet-fake",
	}

	return FetchVNetLocation(context.Background(), cfg, env.Cred, env.ARM)
}

// TestFetchVNetLocation_ReturnsLocation checks that the region of the VNet is returned.
func TestFetchVNetLocation_ReturnsLocation(t *testing.T) {
	got, err := fetchFakeVNetLocation(t, "swedencentral")
	if err != nil {
		t.Fatalf("FetchVNetLocation returned error: %v", err)
	}
	if got != "swedencentral" {
		t.Fatalf("FetchVNetLocation = %q, want %q", got, "swedencentral")
	}
}

// TestFetchVNetLocation_EmptyLocationIsError checks that a VNet without a region is an error.
func TestFetchVNetLocation_EmptyLocationIsError(t *testing.T) {
	_, err := fetchFakeVNetLocation(t, "")
	if !errors.Is(err, ErrEmptyVNetLocation) {
		t.Fatalf("expected ErrEmptyVNetLocation, got %v", err)
	}
}

// TestFetchVNetLocation_NilConfig checks that a nil config is an error.
func TestFetchVNetLocation_NilConfig(t *testing.T) {
	env := azfakes.NewNetworkEnv(nil, azfakes.VirtualNetworksServerWithLocation("norwayeast"))
	if _, err := FetchVNetLocation(context.Background(), nil, env.Cred, env.ARM); err == nil {
		t.Fatalf("expected error for nil config, got nil")
	}
}

// TestNormalizeLocation checks that a display name becomes the canonical region.
func TestNormalizeLocation(t *testing.T) {
	if got := normalizeLocation(" Norway East "); got != "norwayeast" {
		t.Fatalf("normalizeLocation = %q, want %q", got, "norwayeast")
	}
}
