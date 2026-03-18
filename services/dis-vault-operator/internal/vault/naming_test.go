package vault

import (
	"regexp"
	"testing"
)

func TestDeterministicAzureVaultName(t *testing.T) {
	t.Parallel()

	nameA := DeterministicAzureVaultName("default", "my-app-vault", "prod")
	nameB := DeterministicAzureVaultName("default", "my-app-vault", "prod")
	if nameA != nameB {
		t.Fatalf("TODO: expected deterministic output, got %q and %q", nameA, nameB)
	}

	if len(nameA) == 0 || len(nameA) > 24 {
		t.Fatalf("TODO: expected AKV name length 1..24, got %d (%q)", len(nameA), nameA)
	}

	if matched := regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(nameA); !matched {
		t.Fatalf("TODO: expected AKV-safe characters [a-z0-9-], got %q", nameA)
	}

	if !regexp.MustCompile(`.*-[a-f0-9]{6,8}$`).MatchString(nameA) {
		t.Fatalf("TODO: expected stable hash suffix, got %q", nameA)
	}
}
