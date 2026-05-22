package vault

import (
	"fmt"
	"strings"
)

// DeterministicAzureVaultName returns a deterministic Azure name for a Vault resource.
// The resulting name is lowercase, AKV-safe, <= 24 chars, and has a stable hash suffix.
func DeterministicAzureVaultName(namespace, name, environment string) string {
	const (
		maxLen  = 24
		hashLen = 8
	)

	base := sanitizeKubernetesName(fmt.Sprintf("%s-%s-%s", namespace, name, environment))
	if base == "" {
		base = "vault"
	}

	hash := stableHexHash(namespace + "/" + name + "/" + environment)[:hashLen]
	maxBaseLen := max(maxLen-len(hash)-1, 1) // one '-'
	base = strings.Trim(base[:min(len(base), maxBaseLen)], "-")
	if base == "" {
		base = "v"
	}

	return base + "-" + hash
}
