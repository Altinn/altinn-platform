package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
)

func (r *DatabaseReconciler) mapApplicationIdentityToDatabases(
	ctx context.Context,
	obj client.Object,
) []ctrl.Request {
	identityName := obj.GetName()
	identityNamespace := obj.GetNamespace()

	var dbList storagev1alpha1.DatabaseList
	if err := r.List(ctx, &dbList, client.InNamespace(identityNamespace)); err != nil {
		return nil
	}

	requests := make([]ctrl.Request, 0)
	for i := range dbList.Items {
		db := dbList.Items[i]
		if databaseReferencesIdentity(&db, identityName) {
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      db.Name,
					Namespace: db.Namespace,
				},
			})
		}
	}

	return requests
}

func databaseReferencesIdentity(db *storagev1alpha1.Database, identityName string) bool {
	if db.Spec.Auth.Admin.Identity.IdentityRef != nil && db.Spec.Auth.Admin.Identity.IdentityRef.Name == identityName {
		return true
	}
	if db.Spec.Auth.User.Identity.IdentityRef != nil && db.Spec.Auth.User.Identity.IdentityRef.Name == identityName {
		return true
	}
	return false
}
