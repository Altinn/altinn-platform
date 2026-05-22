package database

import (
	"fmt"
	"math"

	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
)

const (
	defaultBackupRetentionDaysNonProd  = 14
	defaultBackupRetentionDaysProd     = 30
	defaultHighAvailabilityEnabledProd = true
)

type Profile struct {
	SkuName  string
	SkuTier  dbforpostgresqlv1.Sku_Tier
	MemoryGB int
	// TODO: Storage will come later in the beta version.
}

// TODO: these profiles need to be defined later
// SKU memory sizes are from Azure Flexible Server compute options/limits:
// https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/concepts-compute
// https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/concepts-limits
var devProfile = Profile{
	SkuName:  "Standard_B1ms",
	SkuTier:  dbforpostgresqlv1.Sku_Tier_Burstable,
	MemoryGB: 2,
}

var prodProfile = Profile{
	SkuName:  "Standard_D4s_v3",
	SkuTier:  dbforpostgresqlv1.Sku_Tier_GeneralPurpose,
	MemoryGB: 16,
}

func GetProfile(serverType string) Profile {
	if isProdServerType(serverType) {
		return prodProfile
	}
	return devProfile
}

func ResolveBackupRetentionDays(serverType string, requested *int) int {
	if requested != nil {
		return *requested
	}

	if isProdServerType(serverType) {
		return defaultBackupRetentionDaysProd
	}
	return defaultBackupRetentionDaysNonProd
}

func ResolveHighAvailabilityEnabled(serverType string, requested *bool) bool {
	if requested != nil {
		return *requested
	}

	if isProdServerType(serverType) {
		return defaultHighAvailabilityEnabledProd
	}
	return false
}

func ResolveHighAvailabilityMode(serverType string, requested *bool) dbforpostgresqlv1.HighAvailability_Mode {
	if ResolveHighAvailabilityEnabled(serverType, requested) {
		return dbforpostgresqlv1.HighAvailability_Mode_ZoneRedundant
	}
	return dbforpostgresqlv1.HighAvailability_Mode_Disabled
}

func isProdServerType(serverType string) bool {
	return serverType == "prod" || serverType == "production"
}

const (
	maxConnectionsLimit = 5000

	// Azure documents this coefficient for max_connections calculation.
	// The documented formula references memory in GiB, but the published values align
	// with applying the coefficient on MiB.
	// https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/param-connections-authentication-connection-settings#max_connections
	maxConnectionsPerMiB = 0.1049164697034809
)

// ResolveMaxConnections returns the Azure maximum max_connections for the given profile.
func ResolveMaxConnections(profile Profile) (int, error) {
	if profile.MemoryGB <= 0 {
		return 0, fmt.Errorf("profile %q has invalid memory size %d GiB", profile.SkuName, profile.MemoryGB)
	}

	// On lower memory SKUs, Azure uses this linear rule.
	if profile.MemoryGB <= 2 {
		return profile.MemoryGB * 25, nil
	}

	estimated := math.Floor(float64(profile.MemoryGB*1024) * maxConnectionsPerMiB)
	if estimated > maxConnectionsLimit {
		return maxConnectionsLimit, nil
	}

	return int(estimated), nil
}
