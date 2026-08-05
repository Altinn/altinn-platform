package vault

import (
	"fmt"
	"strings"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configMapSuffix     = "dis-vault"
	configMapMaxLen     = 63
	ConfigMapKeyAKVName = "AkvName"
	ConfigMapKeyAKVURI  = "AkvUri"
)

func DeterministicConfigMapName(appName string) string {
	appName = sanitizeKubernetesName(appName)
	if appName == "" {
		appName = defaultManagedResourceBaseName
	}

	name := appName + "-" + configMapSuffix
	if len(name) <= configMapMaxLen {
		return name
	}

	hash := stableHexHash(name)[:8]
	maxBase := max(configMapMaxLen-len(configMapSuffix)-len(hash)-2, 1)
	appName = strings.Trim(appName[:min(len(appName), maxBase)], "-")
	if appName == "" {
		appName = "v"
	}

	return appName + "-" + configMapSuffix + "-" + hash
}

func BuildManagedConfigMap(v *vaultv1alpha1.Vault, azureName, vaultURI string) (*corev1.ConfigMap, error) {
	if v == nil {
		return nil, fmt.Errorf("vault must not be nil")
	}
	authReferenceName, err := ActiveAuthReferenceName(v)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(azureName) == "" {
		return nil, fmt.Errorf("azureName must not be empty")
	}
	vaultURI = strings.TrimSpace(vaultURI)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeterministicConfigMapName(authReferenceName),
			Namespace: v.Namespace,
			Labels: map[string]string{
				ManagedResourceOwnerLabel:     v.Name,
				ManagedResourceComponentLabel: ManagedConfigMapComponentValue,
			},
		},
		Data: map[string]string{
			ConfigMapKeyAKVName: azureName,
			ConfigMapKeyAKVURI:  vaultURI,
		},
	}, nil
}
