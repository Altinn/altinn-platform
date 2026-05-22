package redis

import (
	"regexp"
	"testing"
)

func TestDeterministicAzureRedisName(t *testing.T) {
	t.Parallel()

	a := DeterministicAzureRedisName("default", "my-app-cache", "prod")
	b := DeterministicAzureRedisName("default", "my-app-cache", "prod")
	if a != b {
		t.Fatalf("expected deterministic output, got %q and %q", a, b)
	}
	if len(a) == 0 || len(a) > maxAzureRedisNameLen {
		t.Fatalf("expected name length 1..%d, got %d (%q)", maxAzureRedisNameLen, len(a), a)
	}
	if matched := regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(a); !matched {
		t.Fatalf("expected DNS-label-safe characters, got %q", a)
	}
	if !regexp.MustCompile(`.*-[a-f0-9]{6,8}$`).MatchString(a) {
		t.Fatalf("expected stable hash suffix, got %q", a)
	}
}

func TestDeterministicAzureRedisNameUniqueByInputs(t *testing.T) {
	t.Parallel()

	a := DeterministicAzureRedisName("ns1", "name", "dev")
	b := DeterministicAzureRedisName("ns2", "name", "dev")
	if a == b {
		t.Fatalf("expected different namespaces to produce different names, got %q and %q", a, b)
	}
}

func TestDeterministicKubernetesName(t *testing.T) {
	t.Parallel()

	name := DeterministicKubernetesName("my-redis", "pe")
	if name != "my-redis-pe" {
		t.Fatalf("expected suffix concatenation, got %q", name)
	}
}
