package redis

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	defaultManagedResourceBaseName = "redis"
	maxAzureRedisNameLen           = 60
	azureNameHashLen               = 8
)

// DeterministicAzureRedisName returns a deterministic Azure Managed Redis cluster name.
// The name is lowercase, DNS-label safe, <= 60 chars, and has a stable hash suffix.
func DeterministicAzureRedisName(namespace, name, environment string) string {
	base := sanitizeKubernetesName(fmt.Sprintf("%s-%s-%s", namespace, name, environment))
	if base == "" {
		base = defaultManagedResourceBaseName
	}

	hash := stableHexHash(namespace + "/" + name + "/" + environment)[:azureNameHashLen]
	maxBaseLen := max(maxAzureRedisNameLen-len(hash)-1, 1)
	base = strings.Trim(base[:min(len(base), maxBaseLen)], "-")
	if base == "" {
		base = "r"
	}

	return base + "-" + hash
}

// DeterministicKubernetesName returns a Kubernetes-safe deterministic name with a suffix.
func DeterministicKubernetesName(base, suffix string) string {
	base = sanitizeKubernetesName(base)
	if base == "" {
		base = defaultManagedResourceBaseName
	}
	suffix = sanitizeKubernetesName(suffix)
	if suffix == "" {
		suffix = "res"
	}

	out := base + "-" + suffix
	if len(out) <= validation.DNS1123SubdomainMaxLength {
		return out
	}

	hash := stableHexHash(out)[:8]
	maxBase := max(validation.DNS1123SubdomainMaxLength-len(suffix)-len(hash)-2, 1)
	base = strings.Trim(base[:min(len(base), maxBase)], "-")
	if base == "" {
		base = "r"
	}
	return base + "-" + suffix + "-" + hash
}

func sanitizeKubernetesName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))

	lastHyphen := false
	for _, r := range s {
		isLetter := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if isLetter || isDigit {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			b.WriteByte('-')
			lastHyphen = true
		}
	}

	return strings.Trim(b.String(), "-")
}

func stableHexHash(input string) string {
	sum := sha1.Sum([]byte(input))
	return hex.EncodeToString(sum[:])
}
