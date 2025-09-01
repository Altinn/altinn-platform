package secrets

import (
	"context"
	"net/http"
	"testing"

	"github.com/Altinn/altinn-platform/services/lakmus/test/azfakes"
)

func TestListSecrets_PaginatesAndReturnsAll(t *testing.T) {
	t.Parallel()

	s := azfakes.SecretsServerTwoPages()
	e := azfakes.NewEnv(nil, &s)

	got, err := ListSecrets(context.Background(), "https://kv-example.vault.azure.net/", e.Cred, e.Secrets)
	if err != nil {
		t.Fatalf("ListSecrets error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 secrets, got %d", len(got))
	}

	names := map[string]bool{}
	for _, sp := range got {
		if sp == nil || sp.ID == nil {
			t.Fatalf("nil SecretProperties or ID encountered")
		}
		names[sp.ID.Name()] = true
	}
}

func TestListSecrets_EmptyVault_OK(t *testing.T) {
	t.Parallel()
	s := azfakes.SecretsServerEmpty()
	e := azfakes.NewEnv(nil, &s)

	got, err := ListSecrets(context.Background(), "https://kv-empty.vault.azure.net/", e.Cred, e.Secrets)
	if err != nil {
		t.Fatalf("ListSecrets error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 secrets, got %d", len(got))
	}
}

func TestListSecrets_PagerReturnsError(t *testing.T) {
	t.Parallel()
	s := azfakes.SecretsServerError(http.StatusForbidden, "Forbidden")
	e := azfakes.NewEnv(nil, &s)

	_, err := ListSecrets(context.Background(), "https://kv-forbidden.vault.azure.net/", e.Cred, e.Secrets)
	if err == nil {
		t.Fatalf("expected error from ListSecrets, got nil")
	}
}
