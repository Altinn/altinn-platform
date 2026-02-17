package database

import "strings"

const DefaultStorageTier = "P10"

var storageTierOrder = []string{
	"P1",
	"P2",
	"P3",
	"P4",
	"P6",
	"P10",
	"P15",
	"P20",
	"P30",
	"P40",
	"P50",
	"P60",
	"P70",
	"P80",
}

var storageTierRank = func() map[string]int {
	ranks := make(map[string]int, len(storageTierOrder))
	for i, tier := range storageTierOrder {
		ranks[tier] = i
	}
	return ranks
}()

func normalizeStorageTier(tier string) (string, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(tier))
	_, ok := storageTierRank[normalized]
	return normalized, ok
}

func baselineStorageTier(sizeGB int32) string {
	switch {
	case sizeGB <= 4:
		return "P1"
	case sizeGB <= 8:
		return "P2"
	case sizeGB <= 16:
		return "P3"
	case sizeGB <= 32:
		return "P4"
	case sizeGB <= 64:
		return "P6"
	case sizeGB <= 128:
		return "P10"
	case sizeGB <= 256:
		return "P15"
	case sizeGB <= 512:
		return "P20"
	case sizeGB <= 1024:
		return "P30"
	case sizeGB <= 2048:
		return "P40"
	case sizeGB <= 4096:
		return "P50"
	case sizeGB <= 8192:
		return "P60"
	case sizeGB <= 16384:
		return "P70"
	default:
		return "P80"
	}
}

func maxStorageTierForSize(sizeGB int32) string {
	if sizeGB <= 4096 {
		return "P50"
	}
	return "P80"
}

func clampStorageTier(tier, minTier, maxTier string) string {
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

func resolveStorageTier(sizeGB int32, requested *string) string {
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
func ResolveStorageTier(sizeGB int32, requested *string) string {
	return resolveStorageTier(sizeGB, requested)
}
