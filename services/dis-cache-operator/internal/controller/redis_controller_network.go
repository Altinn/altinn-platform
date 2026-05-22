package controller

import (
	"context"
	"fmt"

	redisv1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
	k8sutil "github.com/Altinn/altinn-platform/services/dis-cache-operator/internal/k8s"
	redispkg "github.com/Altinn/altinn-platform/services/dis-cache-operator/internal/redis"
	networkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20240601"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ensureSharedPrivateDNS get-or-creates the shared privatelink.redis.azure.net zone and AKS VNet link.
// These are shared across all Redis CRs and are NOT owner-referenced to any single CR — instead they are
// label-managed via redis.dis.altinn.cloud/managed-by=dis-cache-operator. Spec/label drift on existing
// resources is reconciled on each pass, mirroring the dis-pgsql-operator DNS reconciliation pattern.
func (r *RedisReconciler) ensureSharedPrivateDNS(ctx context.Context, redisObj *redisv1alpha1.Redis) error {
	logger := log.FromContext(ctx).WithValues("redis", types.NamespacedName{Namespace: redisObj.Namespace, Name: redisObj.Name})

	if err := r.ensureSharedDNSZone(ctx, redisObj.Namespace, logger); err != nil {
		return fmt.Errorf("ensure shared DNS zone: %w", err)
	}
	if err := r.ensureSharedVNetLink(ctx, redisObj.Namespace, logger); err != nil {
		return fmt.Errorf("ensure shared VNet link: %w", err)
	}
	return nil
}

func (r *RedisReconciler) ensureSharedDNSZone(ctx context.Context, namespace string, logger logrLogger) error {
	desired := redispkg.BuildSharedPrivateDNSZone(namespace, r.Config)

	current := &networkv1.PrivateDnsZone{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: namespace}, current)
	if err == nil {
		labels, updated := k8sutil.SyncSpecAndLabels(&current.Spec, desired.Spec, current.Labels, desired.Labels)
		if !updated {
			return nil
		}
		current.Labels = labels
		logger.Info("updating shared private DNS zone", "zoneName", desired.Name, "namespace", namespace)
		if err := r.Update(ctx, current); err != nil {
			return fmt.Errorf("update PrivateDnsZone %s/%s: %w", namespace, desired.Name, err)
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get PrivateDnsZone %s/%s: %w", namespace, desired.Name, err)
	}

	logger.Info("creating shared private DNS zone", "zoneName", desired.Name, "namespace", namespace)
	if err := r.Create(ctx, desired); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create PrivateDnsZone %s/%s: %w", namespace, desired.Name, err)
	}
	return nil
}

func (r *RedisReconciler) ensureSharedVNetLink(ctx context.Context, namespace string, logger logrLogger) error {
	desired := redispkg.BuildSharedVNetLink(namespace, r.Config)

	current := &networkv1.PrivateDnsZonesVirtualNetworkLink{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: namespace}, current)
	if err == nil {
		labels, updated := k8sutil.SyncSpecAndLabels(&current.Spec, desired.Spec, current.Labels, desired.Labels)
		if !updated {
			return nil
		}
		current.Labels = labels
		logger.Info("updating shared private DNS VNet link", "linkName", desired.Name, "namespace", namespace)
		if err := r.Update(ctx, current); err != nil {
			return fmt.Errorf("update PrivateDnsZonesVirtualNetworkLink %s/%s: %w", namespace, desired.Name, err)
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get PrivateDnsZonesVirtualNetworkLink %s/%s: %w", namespace, desired.Name, err)
	}

	logger.Info("creating shared private DNS VNet link", "linkName", desired.Name, "namespace", namespace)
	if err := r.Create(ctx, desired); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create PrivateDnsZonesVirtualNetworkLink %s/%s: %w", namespace, desired.Name, err)
	}
	return nil
}

// logrLogger is the minimal interface used by the helpers above.
type logrLogger interface {
	Info(msg string, keysAndValues ...any)
}
