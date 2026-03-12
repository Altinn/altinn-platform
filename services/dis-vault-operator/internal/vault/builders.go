package vault

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"maps"
	"strings"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/config"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	keyvaultv1 "github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// BuildASOKeyVaultResource builds the desired ASO Key Vault resource.
func BuildASOKeyVaultResource(v *vaultv1alpha1.Vault, cfg config.OperatorConfig, azureName string) (*keyvaultv1.Vault, error) {
	if v == nil {
		return nil, fmt.Errorf("vault must not be nil")
	}
	if strings.TrimSpace(azureName) == "" {
		return nil, fmt.Errorf("azureName must not be empty")
	}

	keyVaultK8sName := deterministicKubernetesName(v.Name, "akv")
	location := cfg.Location
	enableRbac := true

	defaultAction := keyvaultv1.NetworkRuleSet_DefaultAction_Deny
	bypass := keyvaultv1.NetworkRuleSet_Bypass_None
	publicNetworkAccess := string(vaultv1alpha1.VaultPublicNetworkAccessEnabled)
	if v.Spec.PublicNetworkAccess != "" {
		publicNetworkAccess = string(v.Spec.PublicNetworkAccess)
	}

	sku := keyvaultv1.Sku_Name_Standard
	if strings.EqualFold(string(v.Spec.SKU), string(vaultv1alpha1.VaultSKUPremium)) {
		sku = keyvaultv1.Sku_Name_Premium
	}
	skuFamily := keyvaultv1.Sku_Family_A

	networkRules := make([]keyvaultv1.VirtualNetworkRule, 0, len(cfg.AKSSubnetIDs))
	for _, subnetID := range cfg.AKSSubnetIDs {
		subnetID = strings.TrimSpace(subnetID)
		if subnetID == "" {
			continue
		}
		networkRules = append(networkRules, keyvaultv1.VirtualNetworkRule{
			Reference: &genruntime.ResourceReference{
				ARMID: subnetID,
			},
		})
	}

	properties := &keyvaultv1.VaultProperties{
		EnableRbacAuthorization: &enableRbac,
		PublicNetworkAccess:     &publicNetworkAccess,
		NetworkAcls: &keyvaultv1.NetworkRuleSet{
			DefaultAction:       &defaultAction,
			Bypass:              &bypass,
			VirtualNetworkRules: networkRules,
		},
		Sku: &keyvaultv1.Sku{
			Name:   &sku,
			Family: &skuFamily,
		},
	}

	// Respect Vault defaults even if they were not applied by API server in tests.
	retentionDays := v.Spec.SoftDeleteRetentionDays
	if retentionDays == 0 {
		retentionDays = 90
	}
	properties.SoftDeleteRetentionInDays = &retentionDays

	purgeProtection := true
	if v.Spec.PurgeProtectionEnabled != nil {
		purgeProtection = *v.Spec.PurgeProtectionEnabled
	}
	properties.EnablePurgeProtection = &purgeProtection
	tags := maps.Clone(v.Spec.Tags)
	if len(tags) == 0 {
		tags = nil
	}

	keyVault := &keyvaultv1.Vault{
		ObjectMeta: metav1.ObjectMeta{
			Name:      keyVaultK8sName,
			Namespace: v.Namespace,
			Labels: map[string]string{
				"vault.dis.altinn.cloud/name": v.Name,
			},
		},
		Spec: keyvaultv1.Vault_Spec{
			AzureName: azureName,
			Location:  &location,
			Owner: &genruntime.KnownResourceReference{
				ARMID: fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", cfg.SubscriptionID, cfg.ResourceGroup),
			},
			Properties: properties,
			Tags:       tags,
		},
	}

	return keyVault, nil
}

// BuildOwnerRoleAssignmentResource builds the desired owner RoleAssignment resource.
func BuildOwnerRoleAssignmentResource(v *vaultv1alpha1.Vault, keyVault *keyvaultv1.Vault, principalID string) (*authorizationv1.RoleAssignment, error) {
	if v == nil {
		return nil, fmt.Errorf("vault must not be nil")
	}
	if strings.TrimSpace(principalID) == "" {
		return nil, fmt.Errorf("principalID must not be empty")
	}

	principalType := authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal
	roleDef := "Key Vault Secrets Officer"
	roleAssignmentName := deterministicKubernetesName(v.Name, "owner-ra")
	azureName := stableHexHash(v.Namespace + "/" + v.Name + "/" + principalID)[:24]

	owner := genruntime.ArbitraryOwnerReference{
		Group: keyvaultv1.GroupVersion.Group,
		Kind:  "Vault",
		Name:  deterministicKubernetesName(v.Name, "akv"),
	}
	if keyVault != nil {
		owner.Name = keyVault.Name
	}

	roleAssignment := &authorizationv1.RoleAssignment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleAssignmentName,
			Namespace: v.Namespace,
			Labels: map[string]string{
				"vault.dis.altinn.cloud/name": v.Name,
			},
		},
		Spec: authorizationv1.RoleAssignment_Spec{
			AzureName:     azureName,
			Owner:         &owner,
			PrincipalId:   &principalID,
			PrincipalType: &principalType,
			RoleDefinitionReference: &genruntime.WellKnownResourceReference{
				WellKnownName: roleDef,
			},
		},
	}

	return roleAssignment, nil
}

func deterministicKubernetesName(base, suffix string) string {
	base = sanitizeKubernetesName(base)
	if base == "" {
		base = "vault"
	}
	suffix = sanitizeKubernetesName(suffix)
	if suffix == "" {
		suffix = "res"
	}

	name := base + "-" + suffix
	if len(name) <= validation.DNS1123SubdomainMaxLength {
		return name
	}

	hash := stableHexHash(name)[:8]
	maxBase := max(validation.DNS1123SubdomainMaxLength-len(suffix)-len(hash)-2, 1) // two '-'
	base = strings.Trim(base[:min(len(base), maxBase)], "-")
	if base == "" {
		base = "v"
	}
	return base + "-" + suffix + "-" + hash
}

func sanitizeKubernetesName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))

	lastHyphen := false
	for _, r := range s {
		isLetter := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if isLetter || isDigit {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			b.WriteByte('-')
			lastHyphen = true
		}
	}

	return strings.Trim(b.String(), "-")
}

func stableHexHash(input string) string {
	sum := sha1.Sum([]byte(input))
	return hex.EncodeToString(sum[:])
}
