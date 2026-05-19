package controller

import (
	"context"

	redisv1alpha1 "github.com/Altinn/altinn-platform/services/dis-redis-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *RedisReconciler) mapApplicationIdentityToRedises(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.mapIdentitySourceToRedises(ctx, obj.GetNamespace(), func(rd *redisv1alpha1.Redis) bool {
		return redisReferencesApplicationIdentity(rd, obj.GetName())
	})
}

func (r *RedisReconciler) mapServiceAccountToRedises(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.mapIdentitySourceToRedises(ctx, obj.GetNamespace(), func(rd *redisv1alpha1.Redis) bool {
		return redisReferencesServiceAccount(rd, obj.GetName())
	})
}

func (r *RedisReconciler) mapIdentitySourceToRedises(
	ctx context.Context,
	namespace string,
	matches func(*redisv1alpha1.Redis) bool,
) []ctrl.Request {
	var list redisv1alpha1.RedisList
	if err := r.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil
	}

	requests := make([]ctrl.Request, 0)
	for i := range list.Items {
		rd := list.Items[i]
		if matches(&rd) {
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: rd.Name, Namespace: rd.Namespace},
			})
		}
	}
	return requests
}

func redisReferencesApplicationIdentity(r *redisv1alpha1.Redis, identityName string) bool {
	return r.Spec.IdentityRef != nil && r.Spec.IdentityRef.Name == identityName
}

func redisReferencesServiceAccount(r *redisv1alpha1.Redis, serviceAccountName string) bool {
	return r.Spec.ServiceAccountRef != nil && r.Spec.ServiceAccountRef.Name == serviceAccountName
}
