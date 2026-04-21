package vault

import (
	"strings"
	"testing"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	externalsecretsv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
)

func TestDeterministicSecretStoreName(t *testing.T) {
	t.Parallel()

	got := DeterministicSecretStoreName("my-app-vault")
	if got != "my-app-vault-secret-store" {
		t.Fatalf("expected preferred SecretStore name, got %q", got)
	}
}

func TestDeterministicSecretStoreNameTruncatesLongNames(t *testing.T) {
	t.Parallel()

	base := strings.Repeat("very-long-vault-name-", 5)
	got := DeterministicSecretStoreName(base)
	got2 := DeterministicSecretStoreName(base)

	if got != got2 {
		t.Fatalf("expected deterministic SecretStore name, got %q and %q", got, got2)
	}
	if len(got) > 63 {
		t.Fatalf("expected DNS-1123 compliant name length, got %d for %q", len(got), got)
	}
	if !strings.Contains(got, "-ss-") {
		t.Fatalf("expected hashed short suffix fallback, got %q", got)
	}
}

func TestBuildManagedSecretStore(t *testing.T) {
	t.Parallel()

	vaultObj := &vaultv1alpha1.Vault{}
	vaultObj.Name = "my-app-vault"
	vaultObj.Namespace = "default"
	vaultObj.Spec.IdentityRef = &vaultv1alpha1.ApplicationIdentityRef{Name: "my-app-identity"}

	store, err := BuildManagedSecretStore(vaultObj, "00000000-0000-0000-0000-000000000000", "https://my-app-vault.vault.azure.net")
	if err != nil {
		t.Fatalf("expected SecretStore builder to succeed, got error: %v", err)
	}
	if store.Name != "my-app-vault-secret-store" {
		t.Fatalf("expected deterministic SecretStore name, got %q", store.Name)
	}
	if store.Namespace != vaultObj.Namespace {
		t.Fatalf("expected namespace %q, got %q", vaultObj.Namespace, store.Namespace)
	}
	if got := store.Labels["vault.dis.altinn.cloud/name"]; got != vaultObj.Name {
		t.Fatalf("expected managed label %q, got %q", vaultObj.Name, got)
	}
	if store.Spec.Provider == nil || store.Spec.Provider.AzureKV == nil {
		t.Fatalf("expected Azure Key Vault provider configuration to be set")
	}
	if store.Spec.Provider.AzureKV.AuthType == nil || *store.Spec.Provider.AzureKV.AuthType != externalsecretsv1.AzureWorkloadIdentity {
		t.Fatalf("expected workload identity auth, got %#v", store.Spec.Provider.AzureKV.AuthType)
	}
	if store.Spec.Provider.AzureKV.ServiceAccountRef == nil || store.Spec.Provider.AzureKV.ServiceAccountRef.Name != vaultObj.Spec.IdentityRef.Name {
		t.Fatalf("expected service account ref %q, got %#v", vaultObj.Spec.IdentityRef.Name, store.Spec.Provider.AzureKV.ServiceAccountRef)
	}
	if store.Spec.Provider.AzureKV.VaultURL == nil || *store.Spec.Provider.AzureKV.VaultURL != "https://my-app-vault.vault.azure.net" {
		t.Fatalf("expected vault URL to be set, got %#v", store.Spec.Provider.AzureKV.VaultURL)
	}
	if store.Spec.Provider.AzureKV.TenantID == nil || *store.Spec.Provider.AzureKV.TenantID != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("expected tenant ID to be set, got %#v", store.Spec.Provider.AzureKV.TenantID)
	}
}

func TestBuildManagedSecretStoreWithServiceAccountRef(t *testing.T) {
	t.Parallel()

	vaultObj := &vaultv1alpha1.Vault{}
	vaultObj.Name = "my-app-vault"
	vaultObj.Namespace = "default"
	vaultObj.Spec.ServiceAccountRef = &vaultv1alpha1.ServiceAccountRef{Name: "my-app-service-account"}

	store, err := BuildManagedSecretStore(vaultObj, "00000000-0000-0000-0000-000000000000", "https://my-app-vault.vault.azure.net")
	if err != nil {
		t.Fatalf("expected SecretStore builder to succeed, got error: %v", err)
	}
	if store.Spec.Provider.AzureKV.ServiceAccountRef == nil || store.Spec.Provider.AzureKV.ServiceAccountRef.Name != "my-app-service-account" {
		t.Fatalf("expected service account ref %q, got %#v", "my-app-service-account", store.Spec.Provider.AzureKV.ServiceAccountRef)
	}
}
