package database

import (
	"fmt"
	"testing"

	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
)

type diskTierConstraint struct {
	sizeGB   int32
	baseline dbforpostgresqlv1.Storage_Tier
	upgrades []dbforpostgresqlv1.Storage_Tier
}

var managedDiskTierConstraints = []diskTierConstraint{
	{
		sizeGB:   4,
		baseline: dbforpostgresqlv1.Storage_Tier_P1,
		upgrades: []dbforpostgresqlv1.Storage_Tier{
			dbforpostgresqlv1.Storage_Tier_P2,
			dbforpostgresqlv1.Storage_Tier_P3,
			dbforpostgresqlv1.Storage_Tier_P4,
			dbforpostgresqlv1.Storage_Tier_P6,
			dbforpostgresqlv1.Storage_Tier_P10,
			dbforpostgresqlv1.Storage_Tier_P15,
			dbforpostgresqlv1.Storage_Tier_P20,
			dbforpostgresqlv1.Storage_Tier_P30,
			dbforpostgresqlv1.Storage_Tier_P40,
			dbforpostgresqlv1.Storage_Tier_P50,
		},
	},
	{
		sizeGB:   8,
		baseline: dbforpostgresqlv1.Storage_Tier_P2,
		upgrades: []dbforpostgresqlv1.Storage_Tier{
			dbforpostgresqlv1.Storage_Tier_P3,
			dbforpostgresqlv1.Storage_Tier_P4,
			dbforpostgresqlv1.Storage_Tier_P6,
			dbforpostgresqlv1.Storage_Tier_P10,
			dbforpostgresqlv1.Storage_Tier_P15,
			dbforpostgresqlv1.Storage_Tier_P20,
			dbforpostgresqlv1.Storage_Tier_P30,
			dbforpostgresqlv1.Storage_Tier_P40,
			dbforpostgresqlv1.Storage_Tier_P50,
		},
	},
	{
		sizeGB:   16,
		baseline: dbforpostgresqlv1.Storage_Tier_P3,
		upgrades: []dbforpostgresqlv1.Storage_Tier{
			dbforpostgresqlv1.Storage_Tier_P4,
			dbforpostgresqlv1.Storage_Tier_P6,
			dbforpostgresqlv1.Storage_Tier_P10,
			dbforpostgresqlv1.Storage_Tier_P15,
			dbforpostgresqlv1.Storage_Tier_P20,
			dbforpostgresqlv1.Storage_Tier_P30,
			dbforpostgresqlv1.Storage_Tier_P40,
			dbforpostgresqlv1.Storage_Tier_P50,
		},
	},
	{
		sizeGB:   32,
		baseline: dbforpostgresqlv1.Storage_Tier_P4,
		upgrades: []dbforpostgresqlv1.Storage_Tier{
			dbforpostgresqlv1.Storage_Tier_P6,
			dbforpostgresqlv1.Storage_Tier_P10,
			dbforpostgresqlv1.Storage_Tier_P15,
			dbforpostgresqlv1.Storage_Tier_P20,
			dbforpostgresqlv1.Storage_Tier_P30,
			dbforpostgresqlv1.Storage_Tier_P40,
			dbforpostgresqlv1.Storage_Tier_P50,
		},
	},
	{
		sizeGB:   64,
		baseline: dbforpostgresqlv1.Storage_Tier_P6,
		upgrades: []dbforpostgresqlv1.Storage_Tier{
			dbforpostgresqlv1.Storage_Tier_P10,
			dbforpostgresqlv1.Storage_Tier_P15,
			dbforpostgresqlv1.Storage_Tier_P20,
			dbforpostgresqlv1.Storage_Tier_P30,
			dbforpostgresqlv1.Storage_Tier_P40,
			dbforpostgresqlv1.Storage_Tier_P50,
		},
	},
	{
		sizeGB:   128,
		baseline: dbforpostgresqlv1.Storage_Tier_P10,
		upgrades: []dbforpostgresqlv1.Storage_Tier{
			dbforpostgresqlv1.Storage_Tier_P15,
			dbforpostgresqlv1.Storage_Tier_P20,
			dbforpostgresqlv1.Storage_Tier_P30,
			dbforpostgresqlv1.Storage_Tier_P40,
			dbforpostgresqlv1.Storage_Tier_P50,
		},
	},
	{
		sizeGB:   256,
		baseline: dbforpostgresqlv1.Storage_Tier_P15,
		upgrades: []dbforpostgresqlv1.Storage_Tier{
			dbforpostgresqlv1.Storage_Tier_P20,
			dbforpostgresqlv1.Storage_Tier_P30,
			dbforpostgresqlv1.Storage_Tier_P40,
			dbforpostgresqlv1.Storage_Tier_P50,
		},
	},
	{
		sizeGB:   512,
		baseline: dbforpostgresqlv1.Storage_Tier_P20,
		upgrades: []dbforpostgresqlv1.Storage_Tier{
			dbforpostgresqlv1.Storage_Tier_P30,
			dbforpostgresqlv1.Storage_Tier_P40,
			dbforpostgresqlv1.Storage_Tier_P50,
		},
	},
	{
		sizeGB:   1024,
		baseline: dbforpostgresqlv1.Storage_Tier_P30,
		upgrades: []dbforpostgresqlv1.Storage_Tier{
			dbforpostgresqlv1.Storage_Tier_P40,
			dbforpostgresqlv1.Storage_Tier_P50,
		},
	},
	{
		sizeGB:   2048,
		baseline: dbforpostgresqlv1.Storage_Tier_P40,
		upgrades: []dbforpostgresqlv1.Storage_Tier{
			dbforpostgresqlv1.Storage_Tier_P50,
		},
	},
	{sizeGB: 4096, baseline: dbforpostgresqlv1.Storage_Tier_P50},
	{
		sizeGB:   8192,
		baseline: dbforpostgresqlv1.Storage_Tier_P60,
		upgrades: []dbforpostgresqlv1.Storage_Tier{
			dbforpostgresqlv1.Storage_Tier_P70,
			dbforpostgresqlv1.Storage_Tier_P80,
		},
	},
	{
		sizeGB:   16384,
		baseline: dbforpostgresqlv1.Storage_Tier_P70,
		upgrades: []dbforpostgresqlv1.Storage_Tier{
			dbforpostgresqlv1.Storage_Tier_P80,
		},
	},
	{sizeGB: 32768, baseline: dbforpostgresqlv1.Storage_Tier_P80},
}

func TestResolveStorageTier_ManagedDiskConstraints(t *testing.T) {
	ptr := func(value string) *string {
		return &value
	}

	for _, row := range managedDiskTierConstraints {
		maxTier := row.baseline
		if len(row.upgrades) > 0 {
			maxTier = row.upgrades[len(row.upgrades)-1]
		}

		rowName := fmt.Sprintf("sizeGB=%d", row.sizeGB)

		t.Run(rowName+"/accepts_all_allowed_tiers", func(t *testing.T) {
			allowed := append([]dbforpostgresqlv1.Storage_Tier{row.baseline}, row.upgrades...)
			for _, tier := range allowed {
				if got := ResolveStorageTier(row.sizeGB, ptr(string(tier))); got != tier {
					t.Fatalf("ResolveStorageTier(%d, %q) = %q, want %q", row.sizeGB, tier, got, tier)
				}
			}
		})

		t.Run(rowName+"/clamps_tiers_below_baseline", func(t *testing.T) {
			baselineRank := storageTierRank[row.baseline]
			for _, tier := range storageTierOrder[:baselineRank] {
				if got := ResolveStorageTier(row.sizeGB, ptr(string(tier))); got != row.baseline {
					t.Fatalf("ResolveStorageTier(%d, %q) = %q, want baseline %q", row.sizeGB, tier, got, row.baseline)
				}
			}
		})

		t.Run(rowName+"/clamps_tiers_above_max", func(t *testing.T) {
			maxRank := storageTierRank[maxTier]
			if maxRank == len(storageTierOrder)-1 {
				return
			}
			for _, tier := range storageTierOrder[maxRank+1:] {
				if got := ResolveStorageTier(row.sizeGB, ptr(string(tier))); got != maxTier {
					t.Fatalf("ResolveStorageTier(%d, %q) = %q, want max %q", row.sizeGB, tier, got, maxTier)
				}
			}
		})

		t.Run(rowName+"/default_and_invalid_follow_same_clamp", func(t *testing.T) {
			want := clampStorageTier(DefaultStorageTier, row.baseline, maxTier)

			if got := ResolveStorageTier(row.sizeGB, nil); got != want {
				t.Fatalf("ResolveStorageTier(%d, nil) = %q, want %q", row.sizeGB, got, want)
			}

			invalid := "P999"
			if got := ResolveStorageTier(row.sizeGB, &invalid); got != want {
				t.Fatalf("ResolveStorageTier(%d, %q) = %q, want %q", row.sizeGB, invalid, got, want)
			}
		})
	}
}
