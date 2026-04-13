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
	identityPending bool,
	principalID string,
	keyVault *keyvaultv1.Vault,
	keyVaultReady vaultpkg.ASOReadyCondition,
	roleAssignment *authorizationv1.RoleAssignment,
	roleAssignmentReady vaultpkg.ASOReadyCondition,
	groupRoleAssignmentCondition metav1.Condition,
	secretStore secretStoreReconcileResult,
) error {
	updated := false

	applyCondition := func(condition metav1.Condition) metav1.Condition {
		if setStatusCondition(vaultObj, condition) {
			updated = true
		}
		return condition
	}
	setString := func(field *string, value string) {
		if *field != value {
			*field = value
			updated = true
		}
	}
	setInt64 := func(field *int64, value int64) {
		if *field != value {
			*field = value
			updated = true
		}
	}

	identityCondition := applyCondition(buildIdentityCondition(vaultObj, identityPending))
	vaultCondition := applyCondition(asoToStatusCondition(
		vaultObj.Generation,
		vaultv1alpha1.ConditionVaultReady,
		keyVaultReady,
		"VaultNotReady",
		"waiting for ASO Key Vault readiness",
	))
	roleCondition := applyCondition(buildOwnerRoleAssignmentCondition(vaultObj, identityPending, roleAssignmentReady))
	groupRoleAssignmentCondition = applyCondition(groupRoleAssignmentCondition)
	networkCondition := applyCondition(buildNetworkPolicyCondition(vaultObj.Generation, desiredKeyVault, r.Config))
	secretStoreCondition := applyCondition(secretStore.Condition)
	applyCondition(vaultpkg.AggregateReadyCondition(
		vaultObj.Generation,
		identityCondition,
		vaultCondition,
		roleCondition,
		networkCondition,
		secretStoreCondition,
		groupRoleAssignmentCondition,
	))

	setString(&vaultObj.Status.AzureName, azureName)
	if identityPending {
		principalID = ""
	}
	setString(&vaultObj.Status.OwnerPrincipalID, principalID)
	setString(&vaultObj.Status.ResourceID, resourceIDFromStatus(keyVault))
	setString(&vaultObj.Status.VaultURI, vaultURIFromStatus(keyVault))
	if identityPending {
		roleAssignment = nil
	}
	setString(&vaultObj.Status.OwnerRoleAssignmentID, roleAssignmentIDFromStatus(roleAssignment))
	setString(&vaultObj.Status.ExternalSecretStoreName, secretStore.Name)
	setInt64(&vaultObj.Status.ObservedGeneration, vaultObj.Generation)

	if !updated {
		return nil
	}
	return r.Status().Update(ctx, vaultObj)
}

func buildIdentityCondition(vaultObj *vaultv1alpha1.Vault, identityPending bool) metav1.Condition {
	if identityPending {
		return vaultpkg.NewCondition(
			vaultv1alpha1.ConditionIdentityReady,
			vaultObj.Generation,
			metav1.ConditionFalse,
			"IdentityNotReady",
			fmt.Sprintf("ApplicationIdentity %q is not ready", vaultObj.Spec.IdentityRef.Name),
		)
	}

	return vaultpkg.NewCondition(
		vaultv1alpha1.ConditionIdentityReady,
		vaultObj.Generation,
		metav1.ConditionTrue,
		"IdentityReady",
		fmt.Sprintf("ApplicationIdentity %q is ready", vaultObj.Spec.IdentityRef.Name),
	)
}

func buildOwnerRoleAssignmentCondition(
	vaultObj *vaultv1alpha1.Vault,
	identityPending bool,
	roleAssignmentReady vaultpkg.ASOReadyCondition,
) metav1.Condition {
	if identityPending {
		return vaultpkg.NewCondition(
			vaultv1alpha1.ConditionRoleAssignmentReady,
			vaultObj.Generation,
			metav1.ConditionFalse,
			"IdentityNotReady",
			fmt.Sprintf("waiting for ApplicationIdentity %q before reconciling owner role assignment", vaultObj.Spec.IdentityRef.Name),
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
