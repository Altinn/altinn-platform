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
	esov1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

type secretStoreReconcileResult struct {
	Condition metav1.Condition
	Name      string
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
	}

	keyVault, keyVaultReady, err := r.getCurrentKeyVault(ctx, desiredKeyVault.Name, vaultObj.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	roleAssignment, roleAssignmentReady, err := r.getCurrentRoleAssignment(
		ctx,
		vaultpkg.BuildOwnerRoleAssignmentName(vaultObj.Name),
		vaultObj.Namespace,
	)
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

	if _, err := mgr.GetRESTMapper().RESTMapping(
		schema.GroupKind{Group: esov1.Group, Kind: esov1.SecretStoreKind},
		esov1.Version,
	); err == nil {
		builder = builder.Owns(&esov1.SecretStore{})
	} else if !apimeta.IsNoMatchError(err) {
		return err
	}

	return builder.Complete(r)
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

func (r *VaultReconciler) upsertManagedSecretStore(ctx context.Context, owner *vaultv1alpha1.Vault, desired *esov1.SecretStore) error {
	current := &esov1.SecretStore{}
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

func (r *VaultReconciler) reconcileManagedSecretStore(
	ctx context.Context,
	vaultObj *vaultv1alpha1.Vault,
	keyVault *keyvaultv1.Vault,
) (secretStoreReconcileResult, error) {
	name := vaultpkg.DeterministicSecretStoreName(vaultObj.Name)
	key := types.NamespacedName{Name: name, Namespace: vaultObj.Namespace}

	if !vaultObj.Spec.ExternalSecrets {
		if err := r.deleteManagedSecretStore(ctx, vaultObj, key); err != nil {
			return secretStoreReconcileResult{}, err
		}
		return secretStoreReconcileResult{
			Condition: vaultpkg.NewCondition(
				vaultv1alpha1.ConditionExternalSecretsReady,
				vaultObj.Generation,
				metav1.ConditionFalse,
				"Disabled",
				"external secrets integration is disabled",
			),
		}, nil
	}

	current := &esov1.SecretStore{}
	if err := r.Get(ctx, key, current); err != nil {
		switch {
		case apierrors.IsNotFound(err):
			current = nil
		case apimeta.IsNoMatchError(err):
			return secretStoreReconcileResult{
				Condition: vaultpkg.NewCondition(
					vaultv1alpha1.ConditionExternalSecretsReady,
					vaultObj.Generation,
					metav1.ConditionFalse,
					"CRDNotInstalled",
					"SecretStore CRD is not installed in the cluster",
				),
			}, nil
		default:
			return secretStoreReconcileResult{}, err
		}
	}

	if current != nil && !metav1.IsControlledBy(current, vaultObj) {
		return secretStoreReconcileResult{
			Condition: vaultpkg.NewCondition(
				vaultv1alpha1.ConditionExternalSecretsReady,
				vaultObj.Generation,
				metav1.ConditionFalse,
				"NameConflict",
				fmt.Sprintf("SecretStore %q already exists and is not managed by this Vault", name),
			),
		}, nil
	}

	vaultURI := vaultURIFromStatus(keyVault)
	if vaultURI == "" {
		if current != nil {
			return secretStoreReconcileResult{
				Name: current.Name,
				Condition: vaultpkg.NewCondition(
					vaultv1alpha1.ConditionExternalSecretsReady,
					vaultObj.Generation,
					metav1.ConditionTrue,
					"Ready",
					"managed SecretStore is present",
				),
			}, nil
		}
		return secretStoreReconcileResult{
			Condition: vaultpkg.NewCondition(
				vaultv1alpha1.ConditionExternalSecretsReady,
				vaultObj.Generation,
				metav1.ConditionUnknown,
				"VaultNotReady",
				"waiting for Vault URI before reconciling SecretStore",
			),
		}, nil
	}

	desired, err := vaultpkg.BuildManagedSecretStore(vaultObj, r.Config.TenantID, vaultURI)
	if err != nil {
		return secretStoreReconcileResult{}, err
	}
	if err := ctrl.SetControllerReference(vaultObj, desired, r.Scheme); err != nil {
		return secretStoreReconcileResult{}, err
	}
	if err := r.upsertManagedSecretStore(ctx, vaultObj, desired); err != nil {
		if apimeta.IsNoMatchError(err) {
			return secretStoreReconcileResult{
				Condition: vaultpkg.NewCondition(
					vaultv1alpha1.ConditionExternalSecretsReady,
					vaultObj.Generation,
					metav1.ConditionFalse,
					"CRDNotInstalled",
					"SecretStore CRD is not installed in the cluster",
				),
			}, nil
		}
		return secretStoreReconcileResult{}, err
	}

	return secretStoreReconcileResult{
		Name: desired.Name,
		Condition: vaultpkg.NewCondition(
			vaultv1alpha1.ConditionExternalSecretsReady,
			vaultObj.Generation,
			metav1.ConditionTrue,
			"Ready",
			"managed SecretStore reconciled",
		),
	}, nil
}

func (r *VaultReconciler) deleteManagedSecretStore(
	ctx context.Context,
	owner *vaultv1alpha1.Vault,
	key types.NamespacedName,
) error {
	current := &esov1.SecretStore{}
	if err := r.Get(ctx, key, current); err != nil {
		if apierrors.IsNotFound(err) || apimeta.IsNoMatchError(err) {
			return nil
		}
		return err
	}
	if !metav1.IsControlledBy(current, owner) {
		return nil
	}
	return client.IgnoreNotFound(r.Delete(ctx, current))
}

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
	secretStore secretStoreReconcileResult,
) error {
	updated := false

	identityCondition := vaultpkg.NewCondition(
		vaultv1alpha1.ConditionIdentityReady,
		vaultObj.Generation,
		metav1.ConditionTrue,
		"IdentityReady",
		fmt.Sprintf("ApplicationIdentity %q is ready", vaultObj.Spec.IdentityRef.Name),
	)
	if identityPending {
		identityCondition = vaultpkg.NewCondition(
			vaultv1alpha1.ConditionIdentityReady,
			vaultObj.Generation,
			metav1.ConditionFalse,
			"IdentityNotReady",
			fmt.Sprintf("ApplicationIdentity %q is not ready", vaultObj.Spec.IdentityRef.Name),
		)
	}
	if setStatusCondition(vaultObj, identityCondition) {
		updated = true
	}

	vaultCondition := asoToStatusCondition(
		vaultObj.Generation,
		vaultv1alpha1.ConditionVaultReady,
		keyVaultReady,
		"VaultNotReady",
		"waiting for ASO Key Vault readiness",
	)
	if setStatusCondition(vaultObj, vaultCondition) {
		updated = true
	}

	roleCondition := asoToStatusCondition(
		vaultObj.Generation,
		vaultv1alpha1.ConditionRoleAssignmentReady,
		roleAssignmentReady,
		"RoleAssignmentNotReady",
		"waiting for ASO RoleAssignment readiness",
	)
	if setStatusCondition(vaultObj, roleCondition) {
		updated = true
	}

	networkCondition := buildNetworkPolicyCondition(vaultObj.Generation, desiredKeyVault, r.Config)
	if setStatusCondition(vaultObj, networkCondition) {
		updated = true
	}
	if setStatusCondition(vaultObj, secretStore.Condition) {
		updated = true
	}

	readyCondition := vaultpkg.AggregateReadyCondition(
		vaultObj.Generation,
		identityCondition,
		vaultCondition,
		roleCondition,
		networkCondition,
		secretStore.Condition,
	)
	if setStatusCondition(vaultObj, readyCondition) {
		updated = true
	}

	if vaultObj.Status.AzureName != azureName {
		vaultObj.Status.AzureName = azureName
		updated = true
	}
	nextPrincipalID := principalID
	if identityPending {
		nextPrincipalID = ""
	}
	if vaultObj.Status.OwnerPrincipalID != nextPrincipalID {
		vaultObj.Status.OwnerPrincipalID = nextPrincipalID
		updated = true
	}

	resourceID := resourceIDFromStatus(keyVault)
	if vaultObj.Status.ResourceID != resourceID {
		vaultObj.Status.ResourceID = resourceID
		updated = true
	}
	vaultURI := vaultURIFromStatus(keyVault)
	if vaultObj.Status.VaultURI != vaultURI {
		vaultObj.Status.VaultURI = vaultURI
		updated = true
	}
	roleAssignmentID := roleAssignmentIDFromStatus(roleAssignment)
	if vaultObj.Status.OwnerRoleAssignmentID != roleAssignmentID {
		vaultObj.Status.OwnerRoleAssignmentID = roleAssignmentID
		updated = true
	}
	if vaultObj.Status.ExternalSecretStoreName != secretStore.Name {
		vaultObj.Status.ExternalSecretStoreName = secretStore.Name
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

func roleAssignmentIDFromStatus(roleAssignment *authorizationv1.RoleAssignment) string {
	if roleAssignment == nil || roleAssignment.Status.Id == nil {
		return ""
	}
	return *roleAssignment.Status.Id
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
