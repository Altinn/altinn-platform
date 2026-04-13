package controller

import (
	"context"
	"strings"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	vaultpkg "github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/vault"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	keyvaultv1 "github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	roleAssignmentLabelName = "vault.dis.altinn.cloud/name"
	roleAssignmentLabelKind = "vault.dis.altinn.cloud/assignment-kind"
	roleAssignmentKindGroup = "group"
)

func (r *VaultReconciler) reconcileOwnerRoleAssignment(
	ctx context.Context,
	vaultObj *vaultv1alpha1.Vault,
	keyVault *keyvaultv1.Vault,
	principalID string,
) error {
	desired, err := vaultpkg.BuildOwnerRoleAssignmentResource(vaultObj, keyVault, principalID)
	if err != nil {
		return err
	}
	return r.upsertRoleAssignment(ctx, vaultObj, desired)
}

func (r *VaultReconciler) upsertRoleAssignment(
	ctx context.Context,
	owner *vaultv1alpha1.Vault,
	desired *authorizationv1.RoleAssignment,
) error {
	current := &authorizationv1.RoleAssignment{}
	current.SetName(desired.GetName())
	current.SetNamespace(desired.GetNamespace())

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, current, func() error {
		current.Labels = mergeStringMaps(current.Labels, desired.Labels)
		current.Spec = desired.Spec
		return ctrl.SetControllerReference(owner, current, r.Scheme)
	})
	return err
}

func (r *VaultReconciler) getOwnerRoleAssignment(
	ctx context.Context,
	vaultObj *vaultv1alpha1.Vault,
) (*authorizationv1.RoleAssignment, vaultpkg.ASOReadyCondition, error) {
	return r.getCurrentRoleAssignment(
		ctx,
		vaultpkg.BuildOwnerRoleAssignmentName(vaultObj.Name),
		vaultObj.Namespace,
	)
}

func (r *VaultReconciler) getCurrentRoleAssignment(
	ctx context.Context,
	name, namespace string,
) (*authorizationv1.RoleAssignment, vaultpkg.ASOReadyCondition, error) {
	current := &authorizationv1.RoleAssignment{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, current); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, vaultpkg.ASOReadyCondition{}, nil
		}
		return nil, vaultpkg.ASOReadyCondition{}, err
	}

	return current, vaultpkg.FromASOConditions(current.Status.Conditions), nil
}

func (r *VaultReconciler) reconcileGroupRoleAssignment(
	ctx context.Context,
	vaultObj *vaultv1alpha1.Vault,
	keyVault *keyvaultv1.Vault,
) error {
	groupObjectID := strings.TrimSpace(vaultObj.Spec.GroupObjectID)
	desiredName := ""
	if groupObjectID != "" {
		desiredName = vaultpkg.BuildGroupRoleAssignmentName(vaultObj.Name)
		desired, err := vaultpkg.BuildGroupRoleAssignmentResource(vaultObj, keyVault, groupObjectID)
		if err != nil {
			return err
		}

		current, err := r.getRoleAssignment(ctx, desired.Name, desired.Namespace)
		if err != nil {
			return err
		}
		if current == nil || metav1.IsControlledBy(current, vaultObj) {
			if err := r.upsertRoleAssignment(ctx, vaultObj, desired); err != nil {
				return err
			}
		}
	}

	currentAssignments, err := r.listManagedGroupRoleAssignments(ctx, vaultObj)
	if err != nil {
		return err
	}

	for i := range currentAssignments {
		current := currentAssignments[i]
		if desiredName != "" && current.Name == desiredName {
			continue
		}
		if err := client.IgnoreNotFound(r.Delete(ctx, &current)); err != nil {
			return err
		}
	}

	return nil
}

func (r *VaultReconciler) listManagedGroupRoleAssignments(
	ctx context.Context,
	vaultObj *vaultv1alpha1.Vault,
) ([]authorizationv1.RoleAssignment, error) {
	var list authorizationv1.RoleAssignmentList
	if err := r.List(
		ctx,
		&list,
		client.InNamespace(vaultObj.Namespace),
		client.MatchingLabels{
			roleAssignmentLabelName: vaultObj.Name,
			roleAssignmentLabelKind: roleAssignmentKindGroup,
		},
	); err != nil {
		return nil, err
	}

	managed := make([]authorizationv1.RoleAssignment, 0, len(list.Items))
	for i := range list.Items {
		item := list.Items[i]
		if !metav1.IsControlledBy(&item, vaultObj) {
			continue
		}
		managed = append(managed, item)
	}

	return managed, nil
}

func (r *VaultReconciler) getRoleAssignment(
	ctx context.Context,
	name, namespace string,
) (*authorizationv1.RoleAssignment, error) {
	current := &authorizationv1.RoleAssignment{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, current); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return current, nil
}

func (r *VaultReconciler) getGroupRoleAssignmentCondition(
	ctx context.Context,
	vaultObj *vaultv1alpha1.Vault,
) (metav1.Condition, error) {
	if strings.TrimSpace(vaultObj.Spec.GroupObjectID) == "" {
		return vaultpkg.NewCondition(
			vaultv1alpha1.ConditionGroupRoleAssignment,
			vaultObj.Generation,
			metav1.ConditionTrue,
			"NotConfigured",
			"no group role assignment is configured",
		), nil
	}

	current, ready, err := r.getCurrentRoleAssignment(
		ctx,
		vaultpkg.BuildGroupRoleAssignmentName(vaultObj.Name),
		vaultObj.Namespace,
	)
	if err != nil {
		return metav1.Condition{}, err
	}
	if current != nil && !metav1.IsControlledBy(current, vaultObj) {
		return vaultpkg.NewCondition(
			vaultv1alpha1.ConditionGroupRoleAssignment,
			vaultObj.Generation,
			metav1.ConditionFalse,
			"NameConflict",
			"group role assignment conflicts with a non-owned resource",
		), nil
	}

	return asoToStatusCondition(
		vaultObj.Generation,
		vaultv1alpha1.ConditionGroupRoleAssignment,
		ready,
		"GroupRoleAssignmentNotReady",
		"waiting for group role assignment readiness",
	), nil
}

func roleAssignmentIDFromStatus(roleAssignment *authorizationv1.RoleAssignment) string {
	if roleAssignment == nil || roleAssignment.Status.Id == nil {
		return ""
	}
	return *roleAssignment.Status.Id
}
