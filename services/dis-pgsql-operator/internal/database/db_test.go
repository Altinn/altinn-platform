package database

import (
	"testing"

	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
)

const legacyProdSku = "Standard_D4s_v3"

// knownSkus holds the tier and the memory of each SKU that a profile lists.
// Values are from https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/concepts-compute
// Add a SKU here before you add it to a profile.
var knownSkus = map[string]struct {
	tier     dbforpostgresqlv1.SkuTier
	memoryGB int
}{
	"Standard_B1ms":     {tier: dbforpostgresqlv1.SkuTier_Burstable, memoryGB: 2},
	legacyProdSku:       {tier: dbforpostgresqlv1.SkuTier_GeneralPurpose, memoryGB: 16},
	"Standard_D4ads_v5": {tier: dbforpostgresqlv1.SkuTier_GeneralPurpose, memoryGB: 16},
}

func TestProfilesListMatchingSkus(t *testing.T) {
	profiles := map[string]Profile{
		"dev":  devProfile,
		"prod": prodProfile,
	}

	for profileName, profile := range profiles {
		t.Run(profileName, func(t *testing.T) {
			if len(profile.SkuNames) == 0 {
				t.Fatalf("profile %q lists no SKUs", profileName)
			}

			seen := map[string]bool{}
			for _, sku := range profile.SkuNames {
				if seen[sku] {
					t.Fatalf("profile %q lists SKU %q twice", profileName, sku)
				}
				seen[sku] = true

				known, ok := knownSkus[sku]
				if !ok {
					t.Fatalf("profile %q lists SKU %q, which knownSkus does not have", profileName, sku)
				}
				if known.tier != profile.SkuTier {
					t.Fatalf("profile %q lists SKU %q with tier %q, want %q", profileName, sku, known.tier, profile.SkuTier)
				}
				if known.memoryGB != profile.MemoryGB {
					t.Fatalf("profile %q lists SKU %q with %d GiB, want %d GiB", profileName, sku, known.memoryGB, profile.MemoryGB)
				}
			}
		})
	}
}

func TestProfileSkuHelpers(t *testing.T) {
	if got := prodProfile.DefaultSkuName(); got != "Standard_D4ads_v5" {
		t.Fatalf("prodProfile.DefaultSkuName() = %q, want %q", got, "Standard_D4ads_v5")
	}
	if !prodProfile.AcceptsSku(legacyProdSku) {
		t.Fatalf("prodProfile.AcceptsSku(%q) = false, want true", legacyProdSku)
	}
	if prodProfile.AcceptsSku("Standard_B1ms") {
		t.Fatalf("prodProfile.AcceptsSku(%q) = true, want false", "Standard_B1ms")
	}
	if got := (Profile{}).DefaultSkuName(); got != "" {
		t.Fatalf("Profile{}.DefaultSkuName() = %q, want empty", got)
	}
}
