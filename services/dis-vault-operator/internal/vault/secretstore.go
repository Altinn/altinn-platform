package vault

import (
	"fmt"
	"strings"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	externalsecretsv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	esometav1 "github.com/external-secrets/external-secrets/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	secretStorePreferredSuffix = "secret-store"
	secretStoreShortSuffix     = "ss"
	secretStoreNameMaxLength   = 63
)

func DeterministicSecretStoreName(base string) string {
	base = sanitizeKubernetesName(base)
	if base == "" {
		base = defaultManagedResourceBaseName
	}

	preferred := base + "-" + secretStorePreferredSuffix
	if len(preferred) <= secretStoreNameMaxLength {
		return preferred
	}

	short := base + "-" + secretStoreShortSuffix
	if len(short) <= secretStoreNameMaxLength {
		return short
	}

	hash := stableHexHash(preferred)[:8]
	maxBase := max(secretStoreNameMaxLength-len(secretStoreShortSuffix)-len(hash)-2, 1)
	base = strings.Trim(base[:min(len(base), maxBase)], "-")
	if base == "" {
		base = "v"
	}

	return base + "-" + secretStoreShortSuffix + "-" + hash
}

func BuildManagedSecretStore(v *vaultv1alpha1.Vault, tenantID, vaultURI string) (*externalsecretsv1.SecretStore, error) {
	if v == nil {
		return nil, fmt.Errorf("vault must not be nil")
	}
	if strings.TrimSpace(v.Spec.IdentityRef.Name) == "" {
		return nil, fmt.Errorf("identityRef.name must not be empty")
	}
	vaultURI = strings.TrimSpace(vaultURI)
	if vaultURI == "" {
		return nil, fmt.Errorf("vaultURI must not be empty")
	}

	name := DeterministicSecretStoreName(v.Name)
	authType := externalsecretsv1.AzureWorkloadIdentity
	vaultURL := vaultURI
	store := &externalsecretsv1.SecretStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: v.Namespace,
			Labels: map[string]string{
				"vault.dis.altinn.cloud/name": v.Name,
			},
		},
		Spec: externalsecretsv1.SecretStoreSpec{
			Provider: &externalsecretsv1.SecretStoreProvider{
				AzureKV: &externalsecretsv1.AzureKVProvider{
					AuthType: &authType,
					VaultURL: &vaultURL,
					ServiceAccountRef: &esometav1.ServiceAccountSelector{
						Name: v.Spec.IdentityRef.Name,
					},
				},
			},
		},
	}
	if tenantID := strings.TrimSpace(tenantID); tenantID != "" {
		store.Spec.Provider.AzureKV.TenantID = &tenantID
	}

	return store, nil
}
