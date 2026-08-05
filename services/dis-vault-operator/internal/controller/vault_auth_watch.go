package controller

import (
	"context"

	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *VaultReconciler) mapApplicationIdentityToVaults(
	ctx context.Context,
	obj client.Object,
) []ctrl.Request {
	return r.mapIdentitySourceToVaults(ctx, obj.GetNamespace(), func(v *vaultv1alpha1.Vault) bool {
		return vaultReferencesApplicationIdentity(v, obj.GetName())
	})
}

func (r *VaultReconciler) mapServiceAccountToVaults(
	ctx context.Context,
	obj client.Object,
) []ctrl.Request {
	return r.mapIdentitySourceToVaults(ctx, obj.GetNamespace(), func(v *vaultv1alpha1.Vault) bool {
		return vaultReferencesServiceAccount(v, obj.GetName())
	})
}

func (r *VaultReconciler) mapIdentitySourceToVaults(
	ctx context.Context,
	namespace string,
	matches func(*vaultv1alpha1.Vault) bool,
) []ctrl.Request {
	var vaultList vaultv1alpha1.VaultList
	if err := r.List(ctx, &vaultList, client.InNamespace(namespace)); err != nil {
		return nil
	}

	requests := make([]ctrl.Request, 0)
	for i := range vaultList.Items {
		v := vaultList.Items[i]
		if matches(&v) {
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      v.Name,
					Namespace: v.Namespace,
				},
			})
		}
	}

	return requests
}

func vaultReferencesApplicationIdentity(v *vaultv1alpha1.Vault, identityName string) bool {
	return v.Spec.IdentityRef != nil && v.Spec.IdentityRef.Name == identityName
}

func vaultReferencesServiceAccount(v *vaultv1alpha1.Vault, serviceAccountName string) bool {
	return v.Spec.ServiceAccountRef != nil && v.Spec.ServiceAccountRef.Name == serviceAccountName
}
