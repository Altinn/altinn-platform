package hmacsig

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

var (
	testKey  = []byte("super-secret-flux-token")
	testBody = []byte(`{"involvedObject":{"kind":"Kustomization","name":"dialogporten-apps"},"reason":"ReconciliationSucceeded"}`)
)

// sign reproduces what Flux's generic-hmac provider puts in X-Signature.
func sign(t *testing.T, body, key []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerify(t *testing.T) {
	valid := sign(t, testBody, testKey)

	tests := []struct {
		name   string
		body   []byte
		header string
		key    []byte
		want   bool
	}{
		{"valid prefixed signature", testBody, "sha256=" + valid, testKey, true},
		{"valid bare hex signature", testBody, valid, testKey, true},
		{"uppercase hex accepted", testBody, "sha256=" + strings.ToUpper(valid), testKey, true},
		{"tampered body", append([]byte("x"), testBody...), "sha256=" + valid, testKey, false},
		{"truncated body", testBody[:len(testBody)-1], "sha256=" + valid, testKey, false},
		{"wrong key", testBody, "sha256=" + sign(t, testBody, []byte("other-key")), testKey, false},
		{"empty header", testBody, "", testKey, false},
		{"only prefix", testBody, "sha256=", testKey, false},
		{"not hex", testBody, "sha256=zzzz", testKey, false},
		{"wrong algorithm prefix", testBody, "sha1=" + valid, testKey, false},
		{"empty key", testBody, "sha256=" + valid, nil, false},
		{"empty body still verifiable", []byte{}, "sha256=" + sign(t, []byte{}, testKey), testKey, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Verify(tt.body, tt.header, tt.key); got != tt.want {
				t.Errorf("Verify(body=%q, header=%q) = %v, want %v", tt.body, tt.header, got, tt.want)
			}
		})
	}
}

func TestVerifyIgnoresSurroundingSpace(t *testing.T) {
	valid := sign(t, testBody, testKey)
	if !Verify(testBody, " sha256="+valid+" ", testKey) {
		t.Error("Verify() rejected a valid signature with surrounding whitespace")
	}
}
