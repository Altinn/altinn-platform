// Package platformtags computes the platform-owned Azure resource tags — the
// RFC 0007 finops base tag set — for Azure resources created by this operator.
package platformtags

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
)

// RepositoryURL is the value of the RFC 0007 `repository` tag: the source
// repository of the code that creates the tags.
const RepositoryURL = "https://github.com/Altinn/altinn-platform"

// productNamespacePrefix is the fleet naming convention for tenant product
// namespaces; the remainder of the namespace name is the product identifier.
const productNamespacePrefix = "product-"

const (
	keyFinopsProduct = "finops_product"
	keyProduct       = "product"
	keyRepository    = "repository"
)

// ParseBase parses the operator's base-tags configuration: a JSON object of
// tag key/value pairs. An empty value disables platform tagging and yields
// (nil, nil). An unsubstituted Flux placeholder (e.g. "${SOME_VAR}") is not
// valid JSON and returns an error; callers are expected to log it and
// continue without platform tags rather than fail startup.
func ParseBase(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var base map[string]string
	if err := json.Unmarshal([]byte(raw), &base); err != nil {
		return nil, fmt.Errorf("base tags must be a JSON object of string values: %w", err)
	}
	for key := range base {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("base tags contain an empty key")
		}
	}
	if len(base) == 0 {
		return nil, nil
	}
	return base, nil
}

// ForNamespace returns the platform tag set for a resource reconciled in the
// given namespace: the base tags with finops_product/product overridden by
// the product namespace name (when the namespace follows the product-<name>
// convention) and repository pinned to this codebase. Returns nil when no
// base tags are configured, keeping the operator's pre-rollout behavior.
func ForNamespace(base map[string]string, namespace string) map[string]string {
	if len(base) == 0 {
		return nil
	}
	tags := maps.Clone(base)
	if product, ok := strings.CutPrefix(namespace, productNamespacePrefix); ok && product != "" {
		tags[keyFinopsProduct] = product
		tags[keyProduct] = product
	}
	tags[keyRepository] = RepositoryURL
	return tags
}

// Merge overlays platform tags onto tags from other sources (tenant-provided
// or resource-specific). Platform keys win: finops tag values are cost data
// owned by the platform, never tenant input. Returns nil when both maps are
// empty.
func Merge(other, platform map[string]string) map[string]string {
	if len(other) == 0 && len(platform) == 0 {
		return nil
	}
	merged := make(map[string]string, len(other)+len(platform))
	maps.Copy(merged, other)
	maps.Copy(merged, platform)
	return merged
}
