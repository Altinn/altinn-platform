package controller

import (
	"context"
	"fmt"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	vaultpkg "github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/vault"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	keyvaultv1 "github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *VaultReconciler) updateStatus(
	ctx context.Context,
	vaultObj *vaultv1alpha1.Vault,
	azureName string,
	desiredKeyVault *keyvaultv1.Vault,
	identity vaultpkg.ResolvedIdentity,
	identityPending bool,
	keyVault *keyvaultv1.Vault,
	keyVaultReady vaultpkg.ASOReadyCondition,
	roleAssignment *authorizationv1.RoleAssignment,
	roleAssignmentReady vaultpkg.ASOReadyCondition,
	groupRoleAssignmentCondition metav1.Condition,
	secretStore secretStoreReconcileResult,
	configMapResult configMapReconcileResult,
) error {
	updated := false

	applyCondition := func(condition metav1.Condition) metav1.Condition {
		if setStatusCondition(vaultObj, condition) {
			updated = true
		}
		return condition
	}

	identityCondition := applyCondition(buildIdentityCondition(vaultObj, identity))
	vaultCondition := applyCondition(asoToStatusCondition(
		vaultObj.Generation,
		vaultv1alpha1.ConditionVaultReady,
		keyVaultReady,
		"VaultNotReady",
		"waiting for ASO Key Vault readiness",
	))
	roleCondition := applyCondition(buildOwnerRoleAssignmentCondition(vaultObj, identity, roleAssignmentReady))
	groupRoleAssignmentCondition = applyCondition(groupRoleAssignmentCondition)
	networkCondition := applyCondition(buildNetworkPolicyCondition(vaultObj.Generation, desiredKeyVault, r.Config))
	secretStoreCondition := applyCondition(secretStore.Condition)
	configMapCondition := applyCondition(configMapResult.Condition)
	applyCondition(vaultpkg.AggregateReadyCondition(
		vaultObj.Generation,
		identityCondition,
		vaultCondition,
		roleCondition,
		networkCondition,
		secretStoreCondition,
		groupRoleAssignmentCondition,
		configMapCondition,
	))

	updated = setIfChanged(&vaultObj.Status.AzureName, azureName) || updated
	principalID := identity.PrincipalID
	if identityPending {
		principalID = ""
	}
	updated = setIfChanged(&vaultObj.Status.OwnerPrincipalID, principalID) || updated
	updated = setIfChanged(&vaultObj.Status.ResourceID, resourceIDFromStatus(keyVault)) || updated
	updated = setIfChanged(&vaultObj.Status.VaultURI, vaultURIFromStatus(keyVault)) || updated
	if identityPending {
		roleAssignment = nil
	}
	updated = setIfChanged(&vaultObj.Status.OwnerRoleAssignmentID, roleAssignmentIDFromStatus(roleAssignment)) || updated
	updated = setIfChanged(&vaultObj.Status.ExternalSecretStoreName, secretStore.Name) || updated
	updated = setIfChanged(&vaultObj.Status.ObservedGeneration, vaultObj.Generation) || updated

	if !updated {
		return nil
	}
	return r.Status().Update(ctx, vaultObj)
}

func buildIdentityCondition(vaultObj *vaultv1alpha1.Vault, identity vaultpkg.ResolvedIdentity) metav1.Condition {
	if identity.IsPending() {
		return vaultpkg.NewCondition(
			vaultv1alpha1.ConditionIdentityReady,
			vaultObj.Generation,
			metav1.ConditionFalse,
			identity.PendingReason,
			identity.PendingMessage,
		)
	}

	return vaultpkg.NewCondition(
		vaultv1alpha1.ConditionIdentityReady,
		vaultObj.Generation,
		metav1.ConditionTrue,
		"IdentityReady",
		fmt.Sprintf("%s is ready", identity.SourceDescription()),
	)
}

func buildOwnerRoleAssignmentCondition(
	vaultObj *vaultv1alpha1.Vault,
	identity vaultpkg.ResolvedIdentity,
	roleAssignmentReady vaultpkg.ASOReadyCondition,
) metav1.Condition {
	if identity.IsPending() {
		return vaultpkg.NewCondition(
			vaultv1alpha1.ConditionRoleAssignmentReady,
			vaultObj.Generation,
			metav1.ConditionFalse,
			identity.PendingReason,
			fmt.Sprintf("waiting for owner identity before reconciling owner role assignment: %s", identity.PendingMessage),
		)
	}

	return asoToStatusCondition(
		vaultObj.Generation,
		vaultv1alpha1.ConditionRoleAssignmentReady,
		roleAssignmentReady,
		"RoleAssignmentNotReady",
		"waiting for ASO RoleAssignment readiness",
	)
}

func setIfChanged[T comparable](field *T, value T) bool {
	if *field == value {
		return false
	}
	*field = value
	return true
}
