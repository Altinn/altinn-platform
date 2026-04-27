package vault

import (
	"testing"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/config"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	keyvaultv1 "github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	"github.com/google/uuid"
)

const (
	testVaultName     = "my-app-vault"
	testNamespace     = "default"
	testIdentityName  = "my-app-identity"
	testGroupObjectID = "11111111-1111-1111-1111-111111111111"
)

func TestBuildASOKeyVaultResource(t *testing.T) {
	t.Parallel()

	subnetID := "/subscriptions/sub-123/resourceGroups/rg-net/providers/Microsoft.Network/virtualNetworks/vnet/subnets/aks-1"
	cfg := config.OperatorConfig{
		SubscriptionID: "sub-123",
		ResourceGroup:  "rg-dis-dev",
		TenantID:       "00000000-0000-0000-0000-000000000000",
		Location:       "westeurope",
		Environment:    "dev",
		AKSSubnetIDs:   []string{subnetID},
	}

	v := &vaultv1alpha1.Vault{}
	v.Name = testVaultName
	v.Namespace = testNamespace
	v.Spec.IdentityRef = &vaultv1alpha1.ApplicationIdentityRef{Name: testIdentityName}

	resource, err := BuildASOKeyVaultResource(v, cfg, "myappdevabc123")
	if err != nil {
		t.Fatalf("expected key vault builder to succeed, got error: %v", err)
	}
	if resource == nil {
		t.Fatalf("expected key vault builder to return resource")
	}

	props := resource.Spec.Properties
	if props == nil {
		t.Fatalf("expected vault properties to be set")
	}
	if props.EnableRbacAuthorization == nil || !*props.EnableRbacAuthorization {
		t.Fatalf("expected EnableRbacAuthorization=true")
	}
	if props.PublicNetworkAccess == nil || *props.PublicNetworkAccess != string(vaultv1alpha1.VaultPublicNetworkAccessEnabled) {
		t.Fatalf("expected PublicNetworkAccess=Enabled")
	}
	if props.NetworkAcls == nil {
		t.Fatalf("expected NetworkAcls to be set")
	}
	if props.NetworkAcls.DefaultAction == nil || *props.NetworkAcls.DefaultAction != keyvaultv1.NetworkRuleSet_DefaultAction_Deny {
		t.Fatalf("expected NetworkAcls.DefaultAction=Deny")
	}
	if props.NetworkAcls.Bypass == nil || *props.NetworkAcls.Bypass != keyvaultv1.NetworkRuleSet_Bypass_None {
		t.Fatalf("expected NetworkAcls.Bypass=None")
	}
	if got := len(props.NetworkAcls.VirtualNetworkRules); got != 1 {
		t.Fatalf("expected one virtual network rule, got %d", got)
	}
	if props.NetworkAcls.VirtualNetworkRules[0].Reference == nil ||
		props.NetworkAcls.VirtualNetworkRules[0].Reference.ARMID != subnetID {
		t.Fatalf("expected first virtual network rule to reference configured subnet")
	}
	if props.EnablePurgeProtection == nil || !*props.EnablePurgeProtection {
		t.Fatalf("expected EnablePurgeProtection=true when spec value is unset")
	}
	if props.TenantId == nil || *props.TenantId != cfg.TenantID {
		t.Fatalf("expected TenantId=%q, got %#v", cfg.TenantID, props.TenantId)
	}
}

func TestBuildASOKeyVaultResourcePreservesExplicitPurgeProtectionFalse(t *testing.T) {
	t.Parallel()

	cfg := config.OperatorConfig{
		SubscriptionID: "sub-123",
		ResourceGroup:  "rg-dis-dev",
		Location:       "westeurope",
	}

	disabled := false
	v := &vaultv1alpha1.Vault{}
	v.Name = testVaultName
	v.Namespace = testNamespace
	v.Spec.IdentityRef = &vaultv1alpha1.ApplicationIdentityRef{Name: testIdentityName}
	v.Spec.PurgeProtectionEnabled = &disabled

	resource, err := BuildASOKeyVaultResource(v, cfg, "myappdevabc123")
	if err != nil {
		t.Fatalf("expected key vault builder to succeed, got error: %v", err)
	}
	if resource == nil || resource.Spec.Properties == nil {
		t.Fatalf("expected key vault properties to be set")
	}
	if resource.Spec.Properties.EnablePurgeProtection == nil {
		t.Fatalf("expected EnablePurgeProtection to be set")
	}
	if *resource.Spec.Properties.EnablePurgeProtection {
		t.Fatalf("expected EnablePurgeProtection=false when explicitly configured")
	}
}

func TestBuildOwnerRoleAssignmentResource(t *testing.T) {
	t.Parallel()

	v := &vaultv1alpha1.Vault{}
	v.Name = testVaultName
	v.Namespace = testNamespace

	roleAssignment, err := BuildOwnerRoleAssignmentResource(v, nil, "principal-123")
	if err != nil {
		t.Fatalf("expected role assignment builder to succeed, got error: %v", err)
	}
	if roleAssignment == nil {
		t.Fatalf("expected role assignment builder to return resource")
	}

	if roleAssignment.Spec.PrincipalType == nil ||
		*roleAssignment.Spec.PrincipalType != authorizationv1.RoleAssignmentProperties_PrincipalType_ServicePrincipal {
		t.Fatalf("expected PrincipalType=ServicePrincipal")
	}
	if roleAssignment.Spec.RoleDefinitionReference == nil ||
		roleAssignment.Spec.RoleDefinitionReference.WellKnownName != keyVaultSecretsOfficerRole {
		t.Fatalf("expected RoleDefinitionReference.WellKnownName=%s", keyVaultSecretsOfficerRole)
	}
	if _, err := uuid.Parse(roleAssignment.Spec.AzureName); err != nil {
		t.Fatalf("expected AzureName to be a GUID, got %q: %v", roleAssignment.Spec.AzureName, err)
	}

	roleAssignment2, err := BuildOwnerRoleAssignmentResource(v, nil, "principal-123")
	if err != nil {
		t.Fatalf("expected second build to succeed, got error: %v", err)
	}
	if roleAssignment.Spec.AzureName != roleAssignment2.Spec.AzureName {
		t.Fatalf("expected deterministic AzureName across builds")
	}
}

func TestBuildOwnerRoleAssignmentResourceUsesAzureKeyVaultNameForAzureNameSeed(t *testing.T) {
	t.Parallel()

	v := &vaultv1alpha1.Vault{}
	v.Name = testVaultName
	v.Namespace = testNamespace

	firstKeyVault := testKeyVaultWithAzureName("product-infopor-f13d1aeb")
	secondKeyVault := testKeyVaultWithAzureName("product-infopor-3b8f3a59")

	first, err := BuildOwnerRoleAssignmentResource(v, firstKeyVault, "principal-123")
	if err != nil {
		t.Fatalf("expected first role assignment build to succeed, got error: %v", err)
	}
	second, err := BuildOwnerRoleAssignmentResource(v, secondKeyVault, "principal-123")
	if err != nil {
		t.Fatalf("expected second role assignment build to succeed, got error: %v", err)
	}

	if first.Spec.AzureName == second.Spec.AzureName {
		t.Fatalf("expected different Azure Key Vault names to produce different role assignment Azure names")
	}
	if first.Spec.Owner == nil || first.Spec.Owner.Name != firstKeyVault.Name {
		t.Fatalf("expected owner reference to use Kubernetes Key Vault name %q, got %#v", firstKeyVault.Name, first.Spec.Owner)
	}
}

func TestBuildOwnerRoleAssignmentResourceRejectsKeyVaultWithoutAzureName(t *testing.T) {
	t.Parallel()

	v := &vaultv1alpha1.Vault{}
	v.Name = testVaultName
	v.Namespace = testNamespace

	keyVault := testKeyVaultWithAzureName(" ")
	if _, err := BuildOwnerRoleAssignmentResource(v, keyVault, "principal-123"); err == nil {
		t.Fatalf("expected error when Key Vault AzureName is empty")
	}
}

func TestBuildGroupRoleAssignmentResource(t *testing.T) {
	t.Parallel()

	v := &vaultv1alpha1.Vault{}
	v.Name = testVaultName
	v.Namespace = testNamespace

	roleAssignment, err := BuildGroupRoleAssignmentResource(v, nil, testGroupObjectID)
	if err != nil {
		t.Fatalf("expected group role assignment builder to succeed, got error: %v", err)
	}
	if roleAssignment == nil {
		t.Fatalf("expected group role assignment builder to return resource")
	}

	if roleAssignment.Spec.PrincipalType == nil ||
		*roleAssignment.Spec.PrincipalType != authorizationv1.RoleAssignmentProperties_PrincipalType_Group {
		t.Fatalf("expected PrincipalType=Group")
	}
	if roleAssignment.Spec.RoleDefinitionReference == nil ||
		roleAssignment.Spec.RoleDefinitionReference.WellKnownName != keyVaultSecretsOfficerRole {
		t.Fatalf("expected RoleDefinitionReference.WellKnownName=%s", keyVaultSecretsOfficerRole)
	}
	if _, err := uuid.Parse(roleAssignment.Spec.AzureName); err != nil {
		t.Fatalf("expected AzureName to be a GUID, got %q: %v", roleAssignment.Spec.AzureName, err)
	}
	if roleAssignment.Name != BuildGroupRoleAssignmentName(testVaultName) {
		t.Fatalf("expected deterministic group role assignment name")
	}
	if roleAssignment.Labels["vault.dis.altinn.cloud/name"] != testVaultName {
		t.Fatalf("expected vault name label to be set")
	}
	if roleAssignment.Labels["vault.dis.altinn.cloud/assignment-kind"] != "group" {
		t.Fatalf("expected group assignment label to be set")
	}

	roleAssignment2, err := BuildGroupRoleAssignmentResource(v, nil, testGroupObjectID)
	if err != nil {
		t.Fatalf("expected second group build to succeed, got error: %v", err)
	}
	if roleAssignment.Spec.AzureName != roleAssignment2.Spec.AzureName {
		t.Fatalf("expected deterministic AzureName across builds")
	}
}

func TestBuildGroupRoleAssignmentResourceUsesAzureKeyVaultNameForAzureNameSeed(t *testing.T) {
	t.Parallel()

	v := &vaultv1alpha1.Vault{}
	v.Name = testVaultName
	v.Namespace = testNamespace

	firstKeyVault := testKeyVaultWithAzureName("product-infopor-f13d1aeb")
	secondKeyVault := testKeyVaultWithAzureName("product-infopor-3b8f3a59")

	first, err := BuildGroupRoleAssignmentResource(v, firstKeyVault, testGroupObjectID)
	if err != nil {
		t.Fatalf("expected first group role assignment build to succeed, got error: %v", err)
	}
	second, err := BuildGroupRoleAssignmentResource(v, secondKeyVault, testGroupObjectID)
	if err != nil {
		t.Fatalf("expected second group role assignment build to succeed, got error: %v", err)
	}

	if first.Spec.AzureName == second.Spec.AzureName {
		t.Fatalf("expected different Azure Key Vault names to produce different group role assignment Azure names")
	}
	if first.Spec.Owner == nil || first.Spec.Owner.Name != firstKeyVault.Name {
		t.Fatalf("expected owner reference to use Kubernetes Key Vault name %q, got %#v", firstKeyVault.Name, first.Spec.Owner)
	}
}

func TestBuildGroupRoleAssignmentResourceRejectsKeyVaultWithoutAzureName(t *testing.T) {
	t.Parallel()

	v := &vaultv1alpha1.Vault{}
	v.Name = testVaultName
	v.Namespace = testNamespace

	keyVault := testKeyVaultWithAzureName(" ")
	if _, err := BuildGroupRoleAssignmentResource(v, keyVault, testGroupObjectID); err == nil {
		t.Fatalf("expected error when Key Vault AzureName is empty")
	}
}

func TestBuildGroupRoleAssignmentResourceIsDeterministicForDifferentGroups(t *testing.T) {
	t.Parallel()

	v := &vaultv1alpha1.Vault{}
	v.Name = testVaultName
	v.Namespace = testNamespace

	first, err := BuildGroupRoleAssignmentResource(v, nil, testGroupObjectID)
	if err != nil {
		t.Fatalf("expected first build to succeed, got error: %v", err)
	}
	second, err := BuildGroupRoleAssignmentResource(v, nil, "22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatalf("expected second build to succeed, got error: %v", err)
	}

	if first.Spec.AzureName == second.Spec.AzureName {
		t.Fatalf("expected different group object IDs to produce different Azure names")
	}
}

func TestBuildGroupRoleAssignmentName(t *testing.T) {
	t.Parallel()

	nameA := BuildGroupRoleAssignmentName(testVaultName)
	nameB := BuildGroupRoleAssignmentName(testVaultName)
	nameC := BuildGroupRoleAssignmentName("another-vault")

	if nameA != nameB {
		t.Fatalf("expected deterministic name generation across calls")
	}
	if nameA == nameC {
		t.Fatalf("expected different vault names to produce different names")
	}
}

func testKeyVaultWithAzureName(azureName string) *keyvaultv1.Vault {
	keyVault := &keyvaultv1.Vault{}
	keyVault.Name = deterministicKubernetesName(testVaultName, "akv")
	keyVault.Namespace = testNamespace
	keyVault.Spec.AzureName = azureName
	return keyVault
}
