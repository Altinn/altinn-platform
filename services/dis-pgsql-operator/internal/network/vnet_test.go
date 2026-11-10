package network

import (
	"context"
	"errors"
	"testing"

	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/test/azfakes"
)

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

func TestFetchVNetLocation_ReturnsLocation(t *testing.T) {
	got, err := fetchFakeVNetLocation(t, "swedencentral")
	if err != nil {
		t.Fatalf("FetchVNetLocation returned error: %v", err)
	}
	if got != "swedencentral" {
		t.Fatalf("FetchVNetLocation = %q, want %q", got, "swedencentral")
	}
}

func TestFetchVNetLocation_EmptyLocationIsError(t *testing.T) {
	_, err := fetchFakeVNetLocation(t, "")
	if !errors.Is(err, ErrEmptyVNetLocation) {
		t.Fatalf("expected ErrEmptyVNetLocation, got %v", err)
	}
}

func TestFetchVNetLocation_NilConfig(t *testing.T) {
	env := azfakes.NewNetworkEnv(nil, azfakes.VirtualNetworksServerWithLocation("norwayeast"))
	if _, err := FetchVNetLocation(context.Background(), nil, env.Cred, env.ARM); err == nil {
		t.Fatalf("expected error for nil config, got nil")
	}
}
