package controller

import (
	"context"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/connection"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileConnectionConfigMaps upserts one connection ConfigMap per service
// principal and deletes any stale ConfigMaps owned by this Database. It is
// called only once the Database is ready, so the published coordinates are
// always complete. A returned error blocks Ready and triggers a requeue.
func (r *DatabaseReconciler) reconcileConnectionConfigMaps(
	ctx context.Context,
	database *storagev1alpha1.Database,
	coords []connection.Coordinates,
) error {
	desiredNames := make(map[string]struct{}, len(coords))
	for i := range coords {
		desired, err := connection.BuildConnectionConfigMap(database, coords[i])
		if err != nil {
			return err
		}
		desiredNames[desired.Name] = struct{}{}
		if err := r.upsertConnectionConfigMap(ctx, database, desired); err != nil {
			return err
		}
	}

	return r.cleanupStaleConnectionConfigMaps(ctx, database, desiredNames)
}

func (r *DatabaseReconciler) upsertConnectionConfigMap(
	ctx context.Context,
	owner *storagev1alpha1.Database,
	desired *corev1.ConfigMap,
) error {
	current := &corev1.ConfigMap{}
	current.SetName(desired.GetName())
	current.SetNamespace(desired.GetNamespace())

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, current, func() error {
		current.Labels = desired.Labels
		current.Data = desired.Data
		current.BinaryData = nil
		return ctrl.SetControllerReference(owner, current, r.Scheme)
	})
	return err
}

// cleanupStaleConnectionConfigMaps deletes connection ConfigMaps that this
// Database owns but no longer desires (a principal removed from spec or flipped
// to a group). It lists by the component label and filters by owner reference,
// so it never depends on the database label value being a legal (<=63 char)
// label and never deletes a ConfigMap it does not own.
func (r *DatabaseReconciler) cleanupStaleConnectionConfigMaps(
	ctx context.Context,
	owner *storagev1alpha1.Database,
	desiredNames map[string]struct{},
) error {
	var configMaps corev1.ConfigMapList
	if err := r.List(
		ctx,
		&configMaps,
		client.InNamespace(owner.Namespace),
		client.MatchingLabels{connection.LabelComponent: connection.ComponentValue},
	); err != nil {
		return err
	}

	for i := range configMaps.Items {
		current := &configMaps.Items[i]
		if _, ok := desiredNames[current.Name]; ok {
			continue
		}
		if !metav1.IsControlledBy(current, owner) {
			continue
		}
		if err := client.IgnoreNotFound(r.Delete(ctx, current)); err != nil {
			return err
		}
	}

	return nil
}
