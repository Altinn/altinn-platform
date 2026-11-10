package network

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"

	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/config"
)

// ErrEmptyVNetLocation reports a DBs VNet that carries no Azure region.
var ErrEmptyVNetLocation = errors.New("virtual network has no location")

// FetchVNetLocation reads the Azure region of the DBs VNet from Azure.
// The operator creates every FlexibleServer in this region.
// Azure requires a VNet-integrated server to be in the region of its VNet.
// The operator calls this once at startup, like FetchSubnetCatalog.
func FetchVNetLocation(
	ctx context.Context,
	cfg *config.OperatorConfig,
	cred azcore.TokenCredential,
	armOpts *arm.ClientOptions,
) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("operator config is nil")
	}

	client, err := armnetwork.NewVirtualNetworksClient(cfg.SubscriptionId, cred, armOpts)
	if err != nil {
		return "", fmt.Errorf("failed to create virtual networks client: %w", err)
	}

	resp, err := client.Get(ctx, cfg.ResourceGroup, cfg.DBVNetName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get virtual network %s/%s: %w", cfg.ResourceGroup, cfg.DBVNetName, err)
	}

	location := ""
	if resp.Location != nil {
		location = normalizeLocation(*resp.Location)
	}
	if location == "" {
		return "", fmt.Errorf("%w: %s/%s", ErrEmptyVNetLocation, cfg.ResourceGroup, cfg.DBVNetName)
	}

	return location, nil
}

// normalizeLocation returns the canonical form of an Azure region name: lowercase, no whitespace.
// The reconciler replaces a stored FlexibleServer spec that differs from the desired one,
// so the region must always have one form.
func normalizeLocation(location string) string {
	return strings.ToLower(strings.Join(strings.Fields(location), ""))
}
