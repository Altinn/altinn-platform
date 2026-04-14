package controller

import (
	"context"
	"fmt"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	vaultpkg "github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/vault"
	keyvaultv1 "github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	esov1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	builderpkg "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type secretStoreReconcileResult struct {
	Condition metav1.Condition
	Name      string
}

func (r *VaultReconciler) completeWithSecretStoreOwnership(mgr ctrl.Manager, builder *builderpkg.Builder) error {
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

func (r *VaultReconciler) upsertManagedSecretStore(
	ctx context.Context,
	owner *vaultv1alpha1.Vault,
	desired *esov1.SecretStore,
) error {
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
