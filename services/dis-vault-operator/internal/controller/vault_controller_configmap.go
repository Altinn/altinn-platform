package controller

import (
	"context"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	vaultpkg "github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/vault"
	keyvaultv1 "github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type configMapReconcileResult struct {
	Condition metav1.Condition
}

func (r *VaultReconciler) upsertManagedConfigMap(
	ctx context.Context,
	owner *vaultv1alpha1.Vault,
	desired *corev1.ConfigMap,
) error {
	current := &corev1.ConfigMap{}
	current.SetName(desired.GetName())
	current.SetNamespace(desired.GetNamespace())

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, current, func() error {
		current.Labels = mergeStringMaps(current.Labels, desired.Labels)
		current.Data = desired.Data
		current.BinaryData = nil
		return ctrl.SetControllerReference(owner, current, r.Scheme)
	})
	return err
}

func (r *VaultReconciler) cleanupManagedConfigMaps(
	ctx context.Context,
	owner *vaultv1alpha1.Vault,
	desiredKey types.NamespacedName,
) error {
	var configMaps corev1.ConfigMapList
	if err := r.List(
		ctx,
		&configMaps,
		client.InNamespace(owner.Namespace),
		client.MatchingLabels{
			vaultpkg.ManagedResourceOwnerLabel:     owner.Name,
			vaultpkg.ManagedResourceComponentLabel: vaultpkg.ManagedConfigMapComponentValue,
		},
	); err != nil {
		return err
	}

	for i := range configMaps.Items {
		current := &configMaps.Items[i]
		if current.Name == desiredKey.Name || !metav1.IsControlledBy(current, owner) {
			continue
		}
		if err := client.IgnoreNotFound(r.Delete(ctx, current)); err != nil {
			return err
		}
	}

	return nil
}

func (r *VaultReconciler) reconcileManagedConfigMap(
	ctx context.Context,
	vaultObj *vaultv1alpha1.Vault,
	azureName string,
	keyVault *keyvaultv1.Vault,
) (configMapReconcileResult, error) {
	name := vaultpkg.DeterministicConfigMapName(vaultObj.Spec.IdentityRef.Name)
	key := types.NamespacedName{Name: name, Namespace: vaultObj.Namespace}

	if err := r.cleanupManagedConfigMaps(ctx, vaultObj, key); err != nil {
		return configMapReconcileResult{}, err
	}

	current := &corev1.ConfigMap{}
	if err := r.Get(ctx, key, current); err != nil {
		if apierrors.IsNotFound(err) {
			current = nil
		} else {
			return configMapReconcileResult{}, err
		}
	}

	if current != nil && !metav1.IsControlledBy(current, vaultObj) {
		return configMapReconcileResult{
			Condition: vaultpkg.NewCondition(
				vaultv1alpha1.ConditionConfigMapReady,
				vaultObj.Generation,
				metav1.ConditionFalse,
				"NameConflict",
				"managed ConfigMap already exists and is not managed by this Vault",
			),
		}, nil
	}

	vaultURI := vaultURIFromStatus(keyVault)
	if vaultURI == "" {
		if current != nil {
			return configMapReconcileResult{
				Condition: vaultpkg.NewCondition(
					vaultv1alpha1.ConditionConfigMapReady,
					vaultObj.Generation,
					metav1.ConditionTrue,
					"Ready",
					"managed ConfigMap is present",
				),
			}, nil
		}

		return configMapReconcileResult{
			Condition: vaultpkg.NewCondition(
				vaultv1alpha1.ConditionConfigMapReady,
				vaultObj.Generation,
				metav1.ConditionUnknown,
				"VaultNotReady",
				"waiting for Vault URI before reconciling ConfigMap",
			),
		}, nil
	}

	desired, err := vaultpkg.BuildManagedConfigMap(vaultObj, azureName, vaultURI)
	if err != nil {
		return configMapReconcileResult{}, err
	}
	if err := r.upsertManagedConfigMap(ctx, vaultObj, desired); err != nil {
		return configMapReconcileResult{}, err
	}

	return configMapReconcileResult{
		Condition: vaultpkg.NewCondition(
			vaultv1alpha1.ConditionConfigMapReady,
			vaultObj.Generation,
			metav1.ConditionTrue,
			"Ready",
			"managed ConfigMap reconciled",
		),
	}, nil
}
