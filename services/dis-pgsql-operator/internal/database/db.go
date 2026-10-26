package database

import (
	"fmt"
	"math"
	"slices"

	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
)

const (
	defaultBackupRetentionDaysNonProd  = 14
	defaultBackupRetentionDaysProd     = 30
	defaultHighAvailabilityEnabledProd = true
)

type Profile struct {
	// SkuNames lists the SKUs that the profile accepts. New servers get the first SKU.
	// A provisioned server keeps its SKU if the SKU is in the list (see ResolveSku).
	// All SKUs in the list must have the tier SkuTier and MemoryGB of memory.
	SkuNames []string
	SkuTier  dbforpostgresqlv1.SkuTier
	MemoryGB int
	// TODO: Storage will come later in the beta version.
}

// TODO: these profiles need to be defined later
// SKU memory sizes are from Azure Flexible Server compute options/limits:
// https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/concepts-compute
// https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/concepts-limits
var devProfile = Profile{
	SkuNames: []string{"Standard_B1ms"},
	SkuTier:  dbforpostgresqlv1.SkuTier_Burstable,
	MemoryGB: 2,
}

// New prod servers get Standard_D4ds_v5.
// Prod servers from earlier operator versions use Standard_D4s_v3.
// Standard_D4s_v3 stays in the list so that the operator does not resize those servers.
// A SKU change restarts a running server.
// Remove Standard_D4s_v3 from the list only to move those servers on purpose.
var prodProfile = Profile{
	SkuNames: []string{"Standard_D4ds_v5", "Standard_D4s_v3"},
	SkuTier:  dbforpostgresqlv1.SkuTier_GeneralPurpose,
	MemoryGB: 16,
}

// DefaultSkuName returns the SKU for new servers.
func (p Profile) DefaultSkuName() string {
	if len(p.SkuNames) == 0 {
		return ""
	}
	return p.SkuNames[0]
}

// AcceptsSku reports whether the profile accepts the SKU for a provisioned server.
func (p Profile) AcceptsSku(name string) bool {
	return slices.Contains(p.SkuNames, name)
}

func GetProfile(serverType string) Profile {
	if isProdServerType(serverType) {
		return prodProfile
	}
	return devProfile
}

// SupportsPgBouncer reports whether the profile's compute tier can run the
// built-in PgBouncer. Azure rejects PgBouncer on the Burstable tier.
func (p Profile) SupportsPgBouncer() bool {
	return p.SkuTier != dbforpostgresqlv1.SkuTier_Burstable
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
		return 0, fmt.Errorf("profile %q has invalid memory size %d GiB", profile.DefaultSkuName(), profile.MemoryGB)
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
