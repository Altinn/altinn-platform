package database

import (
	"strings"

	to "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
)

// A provisioned server keeps its SKU and zones when the defaults change.
// On a running server, a SKU change causes a restart.
// A zone change causes an interruption or an error.
// A server that Azure never provisioned uses the current defaults of the profile.
// The operator does not change the SKU or the zones after a failure.

// ResolveSku returns the compute SKU for a FlexibleServer.
// A provisioned server keeps its current SKU if the profile accepts that SKU.
// Other servers get the default SKU of the profile.
// A change from dev to prod also changes the profile. The prod profile does not
// accept the dev SKU. The server then gets the default prod SKU.
func ResolveSku(profile Profile, current *dbforpostgresqlv1.Sku, provisioned bool) *dbforpostgresqlv1.Sku {
	name := profile.DefaultSkuName()
	if provisioned && current != nil && current.Name != nil {
		if currentName := strings.TrimSpace(*current.Name); profile.AcceptsSku(currentName) {
			name = currentName
		}
	}

	return &dbforpostgresqlv1.Sku{
		Name: to.Ptr(name),
		Tier: to.Ptr(profile.SkuTier),
	}
}

// ResolveZone returns the availability zone for a FlexibleServer or for its standby.
// A provisioned server keeps its current zone. Other servers get the default zone.
func ResolveZone(defaultZone string, current *string, provisioned bool) string {
	if provisioned && current != nil {
		if zone := strings.TrimSpace(*current); zone != "" {
			return zone
		}
	}

	return defaultZone
}
