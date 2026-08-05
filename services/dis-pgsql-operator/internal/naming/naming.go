package naming

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// SanitizeLowerHyphen normalizes a string into lowercase ASCII words separated
// by single hyphens.
func SanitizeLowerHyphen(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))

	var builder strings.Builder
	lastWasSeparator := false
	for _, r := range input {
		if isLowerASCIIAlpha(r) || isASCIIDigit(r) {
			builder.WriteRune(r)
			lastWasSeparator = false
			continue
		}
		if !lastWasSeparator {
			builder.WriteByte('-')
			lastWasSeparator = true
		}
	}

	return strings.Trim(builder.String(), "-")
}

// EnsureLowerAlphaPrefix prepends prefix when name does not start with a
// lowercase ASCII letter.
func EnsureLowerAlphaPrefix(name, prefix string) string {
	if name == "" {
		return prefix
	}
	if isLowerASCIIAlpha(rune(name[0])) {
		return name
	}
	return prefix + "-" + name
}

// StableSHA1Hex returns a deterministic SHA-1 hex digest.
func StableSHA1Hex(input string) string {
	sum := sha1.Sum([]byte(input))
	return hex.EncodeToString(sum[:])
}

// StableSHA256Hex returns a deterministic SHA-256 hex digest.
func StableSHA256Hex(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

// WithHashSuffixOnOverflow returns name unchanged when it fits. Otherwise it
// truncates name and appends "-<hash>".
func WithHashSuffixOnOverflow(name string, maxLen int, hash, fallback string) string {
	if len(name) <= maxLen {
		return name
	}
	return WithRequiredSuffix(name, "-"+hash, maxLen, fallback)
}

// WithRequiredSuffix appends suffix, truncating base when needed to fit maxLen.
func WithRequiredSuffix(base, suffix string, maxLen int, fallback string) string {
	if maxLen <= 0 {
		return ""
	}
	if len(suffix) >= maxLen {
		return suffix[:maxLen]
	}

	maxBaseLen := maxLen - len(suffix)
	if len(base) > maxBaseLen {
		base = strings.TrimRight(base[:maxBaseLen], "-")
		if base == "" {
			base = fallback
		}
	}
	if len(base) > maxBaseLen {
		base = strings.TrimRight(base[:maxBaseLen], "-")
	}
	return base + suffix
}

func isLowerASCIIAlpha(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
