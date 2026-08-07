// Package hmacsig verifies the HMAC-SHA256 signature Flux's generic-hmac
// notification provider puts on every webhook it sends.
package hmacsig

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// signaturePrefix is what Flux prepends to the hex digest in X-Signature.
const signaturePrefix = "sha256="

// Verify reports whether header carries a valid HMAC-SHA256 signature of body
// under key. The header is accepted both as "sha256=<hex>" (what Flux sends)
// and as bare "<hex>". Comparison is constant-time.
func Verify(body []byte, header string, key []byte) bool {
	if len(key) == 0 {
		return false
	}

	digest := strings.TrimSpace(header)
	if strings.Contains(digest, "=") {
		// An algorithm prefix is present; only sha256 is supported.
		if !strings.HasPrefix(digest, signaturePrefix) {
			return false
		}
		digest = strings.TrimPrefix(digest, signaturePrefix)
	}
	if digest == "" {
		return false
	}

	provided, err := hex.DecodeString(strings.ToLower(digest))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	return hmac.Equal(provided, mac.Sum(nil))
}
