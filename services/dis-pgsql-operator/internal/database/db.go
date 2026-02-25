package database

import dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"

const (
	defaultBackupRetentionDaysNonProd  = 14
	defaultBackupRetentionDaysProd     = 30
	defaultHighAvailabilityEnabledProd = true
)

type Profile struct {
	SkuName string
	SkuTier dbforpostgresqlv1.Sku_Tier
	// TODO: Storage will come later in the beta version.
}

// TODO: these profiles need to be defined later
var devProfile = Profile{
	SkuName: "Standard_B1ms",
	SkuTier: dbforpostgresqlv1.Sku_Tier_Burstable,
}

var prodProfile = Profile{
	SkuName: "Standard_D4s_v3",
	SkuTier: dbforpostgresqlv1.Sku_Tier_GeneralPurpose,
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
