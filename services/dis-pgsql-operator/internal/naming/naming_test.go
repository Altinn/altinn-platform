package naming

import (
	"strings"
	"testing"
)

func TestSanitizeLowerHyphen(t *testing.T) {
	got := SanitizeLowerHyphen(" Tenant__123 -- DEV / App DB ")
	if got != "tenant-123-dev-app-db" {
		t.Fatalf("SanitizeLowerHyphen() = %q", got)
	}
}

func TestEnsureLowerAlphaPrefix(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		prefix string
		want   string
	}{
		{name: "keeps letter prefix", input: "tenant123", prefix: "db", want: "tenant123"},
		{name: "prefixes digit", input: "123-tenant", prefix: "db", want: "db-123-tenant"},
		{name: "uses prefix for empty", input: "", prefix: "db", want: "db"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnsureLowerAlphaPrefix(tt.input, tt.prefix)
			if got != tt.want {
				t.Fatalf("EnsureLowerAlphaPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithHashSuffixOnOverflow(t *testing.T) {
	got := WithHashSuffixOnOverflow(strings.Repeat("a", 12), 10, "1234", "x")
	if got != "aaaaa-1234" {
		t.Fatalf("WithHashSuffixOnOverflow() = %q", got)
	}
}

func TestWithRequiredSuffix(t *testing.T) {
	got := WithRequiredSuffix(strings.Repeat("a", 12), "-suffix", 10, "x")
	if got != "aaa-suffix" {
		t.Fatalf("WithRequiredSuffix() = %q", got)
	}
}
