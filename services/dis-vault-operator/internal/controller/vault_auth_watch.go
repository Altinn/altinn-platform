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
	identityName := obj.GetName()
	identityNamespace := obj.GetNamespace()

	var vaultList vaultv1alpha1.VaultList
	if err := r.List(ctx, &vaultList, client.InNamespace(identityNamespace)); err != nil {
		return nil
	}

	requests := make([]ctrl.Request, 0)
	for i := range vaultList.Items {
		v := vaultList.Items[i]
		if vaultReferencesIdentity(&v, identityName) {
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

func vaultReferencesIdentity(v *vaultv1alpha1.Vault, identityName string) bool {
	return v.Spec.IdentityRef.Name == identityName
}
