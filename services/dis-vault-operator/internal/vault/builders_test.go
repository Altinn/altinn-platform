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
	testVaultName    = "my-app-vault"
	testNamespace    = "default"
	testIdentityName = "my-app-identity"
)

func TestBuildASOKeyVaultResource(t *testing.T) {
	t.Parallel()

	subnetID := "/subscriptions/sub-123/resourceGroups/rg-net/providers/Microsoft.Network/virtualNetworks/vnet/subnets/aks-1"
	cfg := config.OperatorConfig{
		SubscriptionID: "sub-123",
		ResourceGroup:  "rg-dis-dev",
		TenantID:       "tenant-123",
		Location:       "westeurope",
		Environment:    "dev",
		AKSSubnetIDs:   []string{subnetID},
	}

	v := &vaultv1alpha1.Vault{}
	v.Name = testVaultName
	v.Namespace = testNamespace
	v.Spec.IdentityRef.Name = testIdentityName

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
	v.Spec.IdentityRef.Name = testIdentityName
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
		roleAssignment.Spec.RoleDefinitionReference.WellKnownName != "Key Vault Secrets Officer" {
		t.Fatalf("expected RoleDefinitionReference.WellKnownName=Key Vault Secrets Officer")
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
