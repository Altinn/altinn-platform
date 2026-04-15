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
	"k8s.io/apimachinery/pkg/types"
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

// External Secrets
// +kubebuilder:rbac:groups=external-secrets.io,resources=secretstores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=external-secrets.io,resources=secretstores/status,verbs=get

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
	desiredKeyVault, err := vaultpkg.BuildASOKeyVaultResource(&vaultObj, r.Config, azureName)
	if err != nil {
		return ctrl.Result{}, err
	}

	identity, identityPending, err := vaultpkg.ResolveOwnerIdentity(
		ctx,
		r.Client,
		vaultObj.Namespace,
		vaultObj.Spec.IdentityRef.Name,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !identityPending {
		if err := r.upsertASOKeyVault(ctx, &vaultObj, desiredKeyVault); err != nil {
			return ctrl.Result{}, err
		}

		ownerReplacementPending, err := r.reconcileOwnerRoleAssignment(ctx, &vaultObj, desiredKeyVault, identity.PrincipalID)
		if err != nil {
			return ctrl.Result{}, err
		}
		if ownerReplacementPending {
			return ctrl.Result{Requeue: true}, nil
		}

		groupReplacementPending, err := r.reconcileGroupRoleAssignment(ctx, &vaultObj, desiredKeyVault)
		if err != nil {
			return ctrl.Result{}, err
		}
		if groupReplacementPending {
			return ctrl.Result{Requeue: true}, nil
		}
	}

	keyVault, keyVaultReady, err := r.getCurrentKeyVault(ctx, desiredKeyVault.Name, vaultObj.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	roleAssignment, roleAssignmentReady, err := r.getOwnerRoleAssignment(ctx, &vaultObj)
	if err != nil {
		return ctrl.Result{}, err
	}
	groupRoleAssignmentCondition, err := r.getGroupRoleAssignmentCondition(ctx, &vaultObj)
	if err != nil {
		return ctrl.Result{}, err
	}
	secretStore, err := r.reconcileManagedSecretStore(ctx, &vaultObj, keyVault)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.updateStatus(
		ctx,
		&vaultObj,
		azureName,
		desiredKeyVault,
		identityPending,
		identity.PrincipalID,
		keyVault,
		keyVaultReady,
		roleAssignment,
		roleAssignmentReady,
		groupRoleAssignmentCondition,
		secretStore,
	); err != nil {
		return ctrl.Result{}, err
	}

	if identityPending {
		return ctrl.Result{RequeueAfter: identityRequeueDelay}, nil
	}

	logger.Info("reconciled Vault dependencies", "azureName", azureName, "principalId", identity.PrincipalID)
	return ctrl.Result{}, nil
}

func (r *VaultReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&vaultv1alpha1.Vault{}).
		Owns(&keyvaultv1.Vault{}).
		Owns(&authorizationv1.RoleAssignment{}).
		Watches(&identityv1alpha1.ApplicationIdentity{}, handler.EnqueueRequestsFromMapFunc(r.mapApplicationIdentityToVaults)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		})

	return r.completeWithSecretStoreOwnership(mgr, builder)
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

func (r *VaultReconciler) getCurrentKeyVault(
	ctx context.Context,
	name, namespace string,
) (*keyvaultv1.Vault, vaultpkg.ASOReadyCondition, error) {
	current := &keyvaultv1.Vault{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, current); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, vaultpkg.ASOReadyCondition{}, nil
		}
		return nil, vaultpkg.ASOReadyCondition{}, err
	}

	return current, vaultpkg.FromASOConditions(current.Status.Conditions), nil
}

func asoToStatusCondition(
	generation int64,
	conditionType vaultv1alpha1.ConditionType,
	input vaultpkg.ASOReadyCondition,
	notReadyReason, notReadyMessage string,
) metav1.Condition {
	if !input.Found {
		return vaultpkg.NewCondition(conditionType, generation, metav1.ConditionUnknown, "NotFound", "dependent resource not found")
	}

	reason := input.Reason
	if reason == "" {
		if input.Status == metav1.ConditionTrue {
			reason = "Ready"
		} else {
			reason = notReadyReason
		}
	}
	message := input.Message
	if message == "" {
		if input.Status == metav1.ConditionTrue {
			message = "dependency is ready"
		} else {
			message = notReadyMessage
		}
	}

	return vaultpkg.NewCondition(conditionType, generation, input.Status, reason, message)
}

func buildNetworkPolicyCondition(
	generation int64,
	desiredKeyVault *keyvaultv1.Vault,
	cfg config.OperatorConfig,
) metav1.Condition {
	if desiredKeyVault == nil || desiredKeyVault.Spec.Properties == nil || desiredKeyVault.Spec.Properties.NetworkAcls == nil {
		return vaultpkg.NewCondition(
			vaultv1alpha1.ConditionNetworkPolicyReady,
			generation,
			metav1.ConditionFalse,
			"InvalidPolicy",
			"Vault network policy could not be rendered",
		)
	}

	props := desiredKeyVault.Spec.Properties
	if err := validateDefaultNetworkPolicy(props, cfg.AKSSubnetIDs); err != nil {
		return vaultpkg.NewCondition(
			vaultv1alpha1.ConditionNetworkPolicyReady,
			generation,
			metav1.ConditionFalse,
			"InvalidPolicy",
			"Vault network policy does not match the RFC defaults",
		)
	}

	return vaultpkg.NewCondition(
		vaultv1alpha1.ConditionNetworkPolicyReady,
		generation,
		metav1.ConditionTrue,
		"Ready",
		"Vault network policy matches the RFC defaults",
	)
}

func validateDefaultNetworkPolicy(props *keyvaultv1.VaultProperties, subnetIDs []string) error {
	switch {
	case props.PublicNetworkAccess == nil:
		return fmt.Errorf("publicNetworkAccess is not set")
	case *props.PublicNetworkAccess != string(vaultv1alpha1.VaultPublicNetworkAccessEnabled):
		return fmt.Errorf("publicNetworkAccess must be %q", vaultv1alpha1.VaultPublicNetworkAccessEnabled)
	case props.NetworkAcls.DefaultAction == nil:
		return fmt.Errorf("networkAcls.defaultAction is not set")
	case *props.NetworkAcls.DefaultAction != keyvaultv1.NetworkRuleSet_DefaultAction_Deny:
		return fmt.Errorf("networkAcls.defaultAction must be %q", keyvaultv1.NetworkRuleSet_DefaultAction_Deny)
	case props.NetworkAcls.Bypass == nil:
		return fmt.Errorf("networkAcls.bypass is not set")
	case *props.NetworkAcls.Bypass != keyvaultv1.NetworkRuleSet_Bypass_None:
		return fmt.Errorf("networkAcls.bypass must be %q", keyvaultv1.NetworkRuleSet_Bypass_None)
	}

	expectedSubnets := countConfiguredSubnets(subnetIDs)
	if len(props.NetworkAcls.VirtualNetworkRules) != expectedSubnets {
		return fmt.Errorf(
			"expected %d virtualNetworkRules entries, got %d",
			expectedSubnets,
			len(props.NetworkAcls.VirtualNetworkRules),
		)
	}

	return nil
}

func countConfiguredSubnets(subnetIDs []string) int {
	count := 0
	for _, subnetID := range subnetIDs {
		if subnetID != "" {
			count++
		}
	}
	return count
}

func resourceIDFromStatus(keyVault *keyvaultv1.Vault) string {
	if keyVault == nil || keyVault.Status.Id == nil {
		return ""
	}
	return *keyVault.Status.Id
}

func vaultURIFromStatus(keyVault *keyvaultv1.Vault) string {
	if keyVault == nil || keyVault.Status.Properties == nil || keyVault.Status.Properties.VaultUri == nil {
		return ""
	}
	return *keyVault.Status.Properties.VaultUri
}

func setStatusCondition(vaultObj *vaultv1alpha1.Vault, condition metav1.Condition) bool {
	return apimeta.SetStatusCondition(&vaultObj.Status.Conditions, condition)
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
