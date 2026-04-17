package vault

import (
	"strings"
	"testing"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
)

func TestDeterministicConfigMapName(t *testing.T) {
	t.Parallel()

	got := DeterministicConfigMapName("my-app")
	if got != "my-app-dis-vault" {
		t.Fatalf("expected preferred ConfigMap name, got %q", got)
	}
}

func TestDeterministicConfigMapNameTruncatesLongNames(t *testing.T) {
	t.Parallel()

	base := strings.Repeat("very-long-application-name-", 4)
	got := DeterministicConfigMapName(base)
	got2 := DeterministicConfigMapName(base)

	if got != got2 {
		t.Fatalf("expected deterministic output ConfigMap name, got %q and %q", got, got2)
	}
	if len(got) > 63 {
		t.Fatalf("expected DNS-1123 compliant name length, got %d for %q", len(got), got)
	}
	if !strings.Contains(got, "-dis-vault-") {
		t.Fatalf("expected hashed dis-vault suffix fallback, got %q", got)
	}
}

func TestBuildManagedConfigMap(t *testing.T) {
	t.Parallel()

	vaultObj := &vaultv1alpha1.Vault{}
	vaultObj.Name = "my-app-vault"
	vaultObj.Namespace = "default"
	vaultObj.Spec.IdentityRef.Name = "my-app"

	configMap, err := BuildManagedConfigMap(vaultObj, "my-akv-name", "https://my-akv.vault.azure.net")
	if err != nil {
		t.Fatalf("expected ConfigMap builder to succeed, got error: %v", err)
	}
	if configMap.Name != "my-app-dis-vault" {
		t.Fatalf("expected deterministic ConfigMap name, got %q", configMap.Name)
	}
	if configMap.Namespace != vaultObj.Namespace {
		t.Fatalf("expected namespace %q, got %q", vaultObj.Namespace, configMap.Namespace)
	}
	if got := configMap.Labels[ManagedResourceOwnerLabel]; got != vaultObj.Name {
		t.Fatalf("expected managed owner label %q, got %q", vaultObj.Name, got)
	}
	if got := configMap.Labels[ManagedResourceComponentLabel]; got != ManagedConfigMapComponentValue {
		t.Fatalf("expected ConfigMap component label %q, got %q", ManagedConfigMapComponentValue, got)
	}
	if got := configMap.Data[ConfigMapKeyAKVName]; got != "my-akv-name" {
		t.Fatalf("expected AkvName data key to be set, got %q", got)
	}
	if got := configMap.Data[ConfigMapKeyAKVURI]; got != "https://my-akv.vault.azure.net" {
		t.Fatalf("expected AkvUri data key to be set, got %q", got)
	}
}
