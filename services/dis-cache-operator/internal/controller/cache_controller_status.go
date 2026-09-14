package controller

import (
	"context"
	"slices"

	valkeyv1alpha1 "github.com/valkey-io/valkey-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
	cachepkg "github.com/Altinn/altinn-platform/services/dis-cache-operator/internal/cache"
)

// Reasons of the Ready condition.
const (
	ReasonProvisioning         = "Provisioning"
	ReasonValkeyReady          = "ValkeyReady"
	ReasonValkeyDegraded       = "ValkeyDegraded"
	ReasonValkeyFailed         = "ValkeyFailed"
	ReasonUpstreamSpecMismatch = "UpstreamSpecMismatch"
	ReasonSecretCreateFailed   = "SecretCreateFailed"
	ReasonApplyFailed          = "ApplyFailed"

	valkeyClientPort = 6379
	// maxMessageLength keeps an error message far below the CRD limit of the
	// condition message.
	maxMessageLength = 1024
)

// writeStatus sets the Ready condition and derives the connection fields from
// it: host and port are set only while the cache is Ready.
func (r *CacheReconciler) writeStatus(ctx context.Context, cache *cachev1alpha1.Cache, condition metav1.Condition) error {
	orig := cache.DeepCopy()

	meta.SetStatusCondition(&cache.Status.Conditions, condition)
	cache.Status.Host, cache.Status.Port = "", 0
	if condition.Status == metav1.ConditionTrue {
		cache.Status.Host = cachepkg.ValkeyServiceName(cache) + "." + cache.Namespace + ".svc.cluster.local"
		cache.Status.Port = valkeyClientPort
	}
	cache.Status.ObservedGeneration = cache.Generation

	return r.patchStatus(ctx, cache, orig)
}

// failed records the error of a reconcile step in the Ready condition and
// returns the error. Without it a Cache that never reaches the ValkeyCluster
// step would show no status at all. The connection fields stay as they are:
// the error is on the operator side, and the Valkey instance is untouched.
func (r *CacheReconciler) failed(ctx context.Context, cache *cachev1alpha1.Cache, reason string, err error) error {
	orig := cache.DeepCopy()

	meta.SetStatusCondition(&cache.Status.Conditions, metav1.Condition{
		Type:               string(cachev1alpha1.ConditionReady),
		Status:             metav1.ConditionFalse,
		ObservedGeneration: cache.Generation,
		Reason:             reason,
		Message:            shortened(err.Error()),
	})
	cache.Status.ObservedGeneration = cache.Generation

	if statusErr := r.patchStatus(ctx, cache, orig); statusErr != nil {
		logf.FromContext(ctx).Error(statusErr, "cannot record the failure in the Cache status")
	}

	return err
}

// patchStatus writes the status when it differs from orig.
func (r *CacheReconciler) patchStatus(ctx context.Context, cache, orig *cachev1alpha1.Cache) error {
	// The client sends an empty patch too, so skip the round trip ourselves.
	if equality.Semantic.DeepEqual(orig.Status, cache.Status) {
		return nil
	}

	// A merge patch carries no resourceVersion, so a concurrent edit of the
	// Cache does not turn into a conflict and a retry.
	return r.Status().Patch(ctx, cache, client.MergeFrom(orig))
}

func shortened(message string) string {
	if len(message) <= maxMessageLength {
		return message
	}

	return message[:maxMessageLength]
}

// readyCondition maps the upstream ValkeyCluster state to the Cache Ready condition.
func readyCondition(generation int64, cluster *valkeyv1alpha1.ValkeyCluster, specMismatch bool) metav1.Condition {
	condition := metav1.Condition{
		Type:               string(cachev1alpha1.ConditionReady),
		ObservedGeneration: generation,
		Status:             metav1.ConditionFalse,
	}

	switch {
	case specMismatch:
		condition.Reason = ReasonUpstreamSpecMismatch
		condition.Message = "the ValkeyCluster users differ from the desired users: an extra or missing user, or a pruned field"
	case cluster.Status.State == valkeyv1alpha1.ClusterStateReady:
		condition.Status = metav1.ConditionTrue
		condition.Reason = ReasonValkeyReady
		condition.Message = "the Valkey instance is ready"
	case cluster.Status.State == valkeyv1alpha1.ClusterStateFailed:
		condition.Reason = ReasonValkeyFailed
		condition.Message = upstreamMessage(cluster)
	case cluster.Status.State == valkeyv1alpha1.ClusterStateDegraded:
		condition.Reason = ReasonValkeyDegraded
		condition.Message = upstreamMessage(cluster)
	default:
		condition.Reason = ReasonProvisioning
		condition.Message = "the valkey-operator is still creating the instance"
	}

	return condition
}

func upstreamMessage(cluster *valkeyv1alpha1.ValkeyCluster) string {
	if cluster.Status.Message != "" {
		return cluster.Status.Message
	}

	return "the valkey-operator reports state " + string(cluster.Status.State)
}

// usersMatch compares the fields of the ACL users that matter for access:
// name, reset flag, password source, and command rules. Defaulted fields
// such as enabled are ignored. A mismatch means the upstream CRD did not keep
// what the operator applied.
func usersMatch(desired, current []valkeyv1alpha1.UserAclSpec) bool {
	if len(desired) != len(current) {
		return false
	}
	for i := range desired {
		d, c := desired[i], current[i]
		if d.Name != c.Name || d.ResetPass != c.ResetPass ||
			d.PasswordSecret.Name != c.PasswordSecret.Name ||
			!slices.Equal(d.PasswordSecret.Keys, c.PasswordSecret.Keys) ||
			!slices.Equal(d.Commands.Allow, c.Commands.Allow) ||
			!slices.Equal(d.Commands.Deny, c.Commands.Deny) {
			return false
		}
	}

	return true
}
