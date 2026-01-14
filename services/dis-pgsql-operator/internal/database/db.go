package database

import dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"

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
	switch serverType {
	case "prod", "production":
		return prodProfile
	case "dev", "development":
		fallthrough
	default:
		return devProfile
	}
}
