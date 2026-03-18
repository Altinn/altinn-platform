package controller

import (
	"context"
	"fmt"
	"maps"
	"time"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/config"
	vaultpkg "github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/vault"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	keyvaultv1 "github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const identityRequeueDelay = 5 * time.Second

// VaultReconciler reconciles a Vault object.
type VaultReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config config.OperatorConfig
}

// +kubebuilder:rbac:groups=vault.dis.altinn.cloud,resources=vaults,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vault.dis.altinn.cloud,resources=vaults/status,verbs=get;update;patch

// ASO: Key Vault
// +kubebuilder:rbac:groups=keyvault.azure.com,resources=vaults,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keyvault.azure.com,resources=vaults/status,verbs=get;update;patch

// ASO: Authorization role assignment
// +kubebuilder:rbac:groups=authorization.azure.com,resources=roleassignments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=authorization.azure.com,resources=roleassignments/status,verbs=get;update;patch

// ApplicationIdentity
// +kubebuilder:rbac:groups=application.dis.altinn.cloud,resources=applicationidentities,verbs=get;list;watch

func (r *VaultReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("vault", req.NamespacedName)

	var vaultObj vaultv1alpha1.Vault
	if err := r.Get(ctx, req.NamespacedName, &vaultObj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !vaultObj.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	azureName := vaultpkg.DeterministicAzureVaultName(vaultObj.Namespace, vaultObj.Name, r.Config.Environment)

	identity, requeue, err := vaultpkg.ResolveOwnerIdentity(
		ctx,
		r.Client,
		vaultObj.Namespace,
		vaultObj.Spec.IdentityRef.Name,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	if requeue {
		if err := r.updateStatusForIdentityNotReady(ctx, &vaultObj, azureName); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: identityRequeueDelay}, nil
	}

	desiredKeyVault, err := vaultpkg.BuildASOKeyVaultResource(&vaultObj, r.Config, azureName)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := ctrl.SetControllerReference(&vaultObj, desiredKeyVault, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.upsertASOKeyVault(ctx, &vaultObj, desiredKeyVault); err != nil {
		return ctrl.Result{}, err
	}

	desiredRoleAssignment, err := vaultpkg.BuildOwnerRoleAssignmentResource(&vaultObj, desiredKeyVault, identity.PrincipalID)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := ctrl.SetControllerReference(&vaultObj, desiredRoleAssignment, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.upsertOwnerRoleAssignment(ctx, &vaultObj, desiredRoleAssignment); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updateStatusForIdentityReady(ctx, &vaultObj, azureName, identity.PrincipalID); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("reconciled Vault dependencies", "azureName", azureName, "principalId", identity.PrincipalID)
	return ctrl.Result{}, nil
}

func (r *VaultReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vaultv1alpha1.Vault{}).
		Owns(&keyvaultv1.Vault{}).
		Owns(&authorizationv1.RoleAssignment{}).
		Watches(&identityv1alpha1.ApplicationIdentity{}, handler.EnqueueRequestsFromMapFunc(r.mapApplicationIdentityToVaults)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

func (r *VaultReconciler) upsertASOKeyVault(ctx context.Context, owner *vaultv1alpha1.Vault, desired *keyvaultv1.Vault) error {
	current := &keyvaultv1.Vault{}
	current.SetName(desired.GetName())
	current.SetNamespace(desired.GetNamespace())

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, current, func() error {
		current.Labels = mergeStringMaps(current.Labels, desired.Labels)
		current.Spec = desired.Spec
		return ctrl.SetControllerReference(owner, current, r.Scheme)
	})
	return err
}

func (r *VaultReconciler) upsertOwnerRoleAssignment(ctx context.Context, owner *vaultv1alpha1.Vault, desired *authorizationv1.RoleAssignment) error {
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

func (r *VaultReconciler) updateStatusForIdentityNotReady(ctx context.Context, vaultObj *vaultv1alpha1.Vault, azureName string) error {
	updated := false
	message := fmt.Sprintf("ApplicationIdentity %q is not ready", vaultObj.Spec.IdentityRef.Name)

	if setCondition(
		vaultObj,
		vaultv1alpha1.ConditionIdentityReady,
		metav1.ConditionFalse,
		"IdentityNotReady",
		message,
	) {
		updated = true
	}

	if vaultObj.Status.AzureName != azureName {
		vaultObj.Status.AzureName = azureName
		updated = true
	}
	if vaultObj.Status.OwnerPrincipalID != "" {
		vaultObj.Status.OwnerPrincipalID = ""
		updated = true
	}
	if vaultObj.Status.ObservedGeneration != vaultObj.Generation {
		vaultObj.Status.ObservedGeneration = vaultObj.Generation
		updated = true
	}

	if !updated {
		return nil
	}
	return r.Status().Update(ctx, vaultObj)
}

func (r *VaultReconciler) updateStatusForIdentityReady(ctx context.Context, vaultObj *vaultv1alpha1.Vault, azureName, principalID string) error {
	updated := false
	message := fmt.Sprintf("ApplicationIdentity %q is ready", vaultObj.Spec.IdentityRef.Name)

	if setCondition(
		vaultObj,
		vaultv1alpha1.ConditionIdentityReady,
		metav1.ConditionTrue,
		"IdentityReady",
		message,
	) {
		updated = true
	}

	if vaultObj.Status.AzureName != azureName {
		vaultObj.Status.AzureName = azureName
		updated = true
	}
	if vaultObj.Status.OwnerPrincipalID != principalID {
		vaultObj.Status.OwnerPrincipalID = principalID
		updated = true
	}
	if vaultObj.Status.ObservedGeneration != vaultObj.Generation {
		vaultObj.Status.ObservedGeneration = vaultObj.Generation
		updated = true
	}

	if !updated {
		return nil
	}
	return r.Status().Update(ctx, vaultObj)
}

func setCondition(vaultObj *vaultv1alpha1.Vault, conditionType vaultv1alpha1.ConditionType, status metav1.ConditionStatus, reason, message string) bool {
	return apimeta.SetStatusCondition(&vaultObj.Status.Conditions, metav1.Condition{
		Type:               string(conditionType),
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: vaultObj.Generation,
	})
}

func mergeStringMaps(existing map[string]string, desired map[string]string) map[string]string {
	if len(existing) == 0 && len(desired) == 0 {
		return nil
	}

	merged := make(map[string]string, len(existing)+len(desired))
	maps.Copy(merged, existing)
	maps.Copy(merged, desired)
	return merged
}
