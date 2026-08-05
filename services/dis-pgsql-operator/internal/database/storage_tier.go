package database

import (
	"strings"

	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
)

const DefaultStorageTier = dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P10

// Tier resolution logic is based on Azure's own rules for disk tier changes:
// https://github.com/MicrosoftDocs/azure-compute-docs/blob/main/articles/virtual-machines/disks-change-performance.md#what-tiers-can-be-changed
var storageTierOrder = []dbforpostgresqlv1.AzureManagedDiskPerformanceTier{
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P1,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P2,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P3,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P4,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P6,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P10,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P15,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P20,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P30,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P40,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P50,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P60,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P70,
	dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P80,
}

var storageTierRank = func() map[dbforpostgresqlv1.AzureManagedDiskPerformanceTier]int {
	ranks := make(map[dbforpostgresqlv1.AzureManagedDiskPerformanceTier]int, len(storageTierOrder))
	for i, tier := range storageTierOrder {
		ranks[tier] = i
	}
	return ranks
}()

func normalizeStorageTier(tier string) (dbforpostgresqlv1.AzureManagedDiskPerformanceTier, bool) {
	normalized := dbforpostgresqlv1.AzureManagedDiskPerformanceTier(strings.ToUpper(strings.TrimSpace(tier)))
	_, ok := storageTierRank[normalized]
	return normalized, ok
}

func baselineStorageTier(sizeGB int32) dbforpostgresqlv1.AzureManagedDiskPerformanceTier {
	switch {
	case sizeGB <= 4:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P1
	case sizeGB <= 8:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P2
	case sizeGB <= 16:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P3
	case sizeGB <= 32:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P4
	case sizeGB <= 64:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P6
	case sizeGB <= 128:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P10
	case sizeGB <= 256:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P15
	case sizeGB <= 512:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P20
	case sizeGB <= 1024:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P30
	case sizeGB <= 2048:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P40
	case sizeGB <= 4096:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P50
	case sizeGB <= 8192:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P60
	case sizeGB <= 16384:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P70
	default:
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P80
	}
}

func maxStorageTierForSize(sizeGB int32) dbforpostgresqlv1.AzureManagedDiskPerformanceTier {
	if sizeGB <= 4096 {
		return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P50
	}
	return dbforpostgresqlv1.AzureManagedDiskPerformanceTier_P80
}

func clampStorageTier(
	tier dbforpostgresqlv1.AzureManagedDiskPerformanceTier,
	minTier dbforpostgresqlv1.AzureManagedDiskPerformanceTier,
	maxTier dbforpostgresqlv1.AzureManagedDiskPerformanceTier,
) dbforpostgresqlv1.AzureManagedDiskPerformanceTier {
	tierRank := storageTierRank[tier]
	minRank := storageTierRank[minTier]
	maxRank := storageTierRank[maxTier]

	if tierRank < minRank {
		return minTier
	}
	if tierRank > maxRank {
		if minRank > maxRank {
			return minTier
		}
		return maxTier
	}
	return tier
}

func resolveStorageTier(sizeGB int32, requested *string) dbforpostgresqlv1.AzureManagedDiskPerformanceTier {
	tier := DefaultStorageTier
	if requested != nil {
		if normalized, ok := normalizeStorageTier(*requested); ok {
			tier = normalized
		}
	}

	baseline := baselineStorageTier(sizeGB)
	maxTier := maxStorageTierForSize(sizeGB)

	return clampStorageTier(tier, baseline, maxTier)
}

// ResolveStorageTier resolves a requested tier into a supported tier based on size.
func ResolveStorageTier(sizeGB int32, requested *string) dbforpostgresqlv1.AzureManagedDiskPerformanceTier {
	return resolveStorageTier(sizeGB, requested)
}
