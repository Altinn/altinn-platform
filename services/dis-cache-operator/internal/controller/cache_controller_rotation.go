package controller

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
	cachepkg "github.com/Altinn/altinn-platform/services/dis-cache-operator/internal/cache"
)

// Reasons of the PasswordRotated condition. Rotating and PasswordCopied are
// the two steps before the new password is written; the reason records which
// Secret patch the operator has confirmed, so a restart resumes at the right
// step without a read of the Secret.
const (
	ReasonRotated               = "Rotated"
	ReasonRotating              = "Rotating"
	ReasonPasswordCopied        = "PasswordCopied"
	ReasonPreviousPasswordValid = "PreviousPasswordValid"
	ReasonRotationFailed        = "RotationFailed"

	// DefaultPreviousValidFor is the period the previous password stays valid
	// when neither the Cache nor the operator flag sets one.
	DefaultPreviousValidFor = 7 * 24 * time.Hour
)

// errPatchRejected means the API server refused a rotation patch because one
// of its test operations failed: the step is not the expected one, or the
// Secret does not carry the Cache label.
var errPatchRejected = errors.New("the rotation patch was rejected")

// rotationDue reports whether the team asked for a rotation that the operator
// has not carried out yet. The schedule comes in a later change.
func rotationDue(cache *cachev1alpha1.Cache) bool {
	spec := cache.Spec.PasswordRotation
	if spec == nil || spec.RequestedAt == nil {
		return false
	}
	status := cache.Status.PasswordRotation
	if status == nil || status.RequestedAt == nil {
		return true
	}

	return spec.RequestedAt.After(status.RequestedAt.Time)
}

// previousValidFor returns the period for this Cache: the spec value, the
// operator flag, or the default.
func (r *CacheReconciler) previousValidFor(cache *cachev1alpha1.Cache) time.Duration {
	if spec := cache.Spec.PasswordRotation; spec != nil && spec.PreviousValidFor != nil {
		return spec.PreviousValidFor.Duration
	}
	if r.PreviousValidFor > 0 {
		return r.PreviousValidFor
	}

	return DefaultPreviousValidFor
}

func (r *CacheReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now().Truncate(time.Second)
	}

	return time.Now().Truncate(time.Second)
}

// rotationCondition returns the PasswordRotated condition of the Cache.
func rotationCondition(cache *cachev1alpha1.Cache) *metav1.Condition {
	return meta.FindStatusCondition(cache.Status.Conditions, string(cachev1alpha1.ConditionPasswordRotated))
}

// setRotationCondition sets the PasswordRotated condition in memory.
func setRotationCondition(cache *cachev1alpha1.Cache, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&cache.Status.Conditions, metav1.Condition{
		Type:               string(cachev1alpha1.ConditionPasswordRotated),
		Status:             status,
		ObservedGeneration: cache.Generation,
		Reason:             reason,
		Message:            message,
	})
}

// authSecretMetadata returns a metadata-only handle of the auth Secret. A
// patch through it carries no Secret data in the response.
func authSecretMetadata(cache *cachev1alpha1.Cache) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: cachepkg.AuthSecretName(cache), Namespace: cache.Namespace},
	}
}

// patchAuthSecret applies a JSON patch to the auth Secret through its
// metadata handle. A rejected patch returns errPatchRejected.
func (r *CacheReconciler) patchAuthSecret(ctx context.Context, cache *cachev1alpha1.Cache, body []byte) error {
	err := r.Patch(ctx, authSecretMetadata(cache), client.RawPatch(types.JSONPatchType, body))
	if err == nil {
		return nil
	}
	if apierrors.IsInvalid(err) {
		return fmt.Errorf("%w: %w", errPatchRejected, err)
	}

	return fmt.Errorf("patch Secret %s: %w", cachepkg.AuthSecretName(cache), err)
}

// markIdle sets the rotation step annotation to idle. It is for Secrets from
// before the annotation existed, and it runs only while no rotation is in
// progress, so idle is the right value in every case.
func (r *CacheReconciler) markIdle(ctx context.Context, cache *cachev1alpha1.Cache) error {
	body, err := cachepkg.RotationMarkIdlePatch()
	if err != nil {
		return err
	}
	if err := r.Patch(ctx, authSecretMetadata(cache), client.RawPatch(types.MergePatchType, body)); err != nil {
		return fmt.Errorf("mark Secret %s idle: %w", cachepkg.AuthSecretName(cache), err)
	}

	return nil
}

// beginRotation runs before the ValkeyCluster apply. It resets the rotation
// state when the Secret was created again, ends the period when it is over,
// and starts a rotation the team asked for. It writes the status before the
// first Secret patch, so the key list of the ValkeyCluster changes first.
func (r *CacheReconciler) beginRotation(ctx context.Context, cache *cachev1alpha1.Cache, secretCreated bool) error {
	logger := logf.FromContext(ctx)
	orig := cache.DeepCopy()
	cond := rotationCondition(cache)
	inProgress := cond != nil && cond.Status == metav1.ConditionFalse

	if secretCreated && inProgress {
		// The Secret came back with one password. The previous key is gone,
		// so the ValkeyCluster must list one key again.
		logger.Info("the auth Secret was created again during a rotation; the rotation state is reset")
		setRotationCondition(cache, metav1.ConditionTrue, ReasonRotated, "the Secret was created again; the rotation was reset")
		if cache.Status.PasswordRotation != nil {
			cache.Status.PasswordRotation.PreviousValidUntil = nil
		}

		return r.patchStatus(ctx, cache, orig)
	}

	if inProgress {
		status := cache.Status.PasswordRotation
		if cond.Reason == ReasonPreviousPasswordValid && status != nil && status.PreviousValidUntil != nil &&
			!r.now().Before(status.PreviousValidUntil.Time) {
			// The period is over. The condition goes True first, so the apply
			// lists one key before the previous password is removed.
			setRotationCondition(cache, metav1.ConditionTrue, ReasonRotated, "the previous password is being removed")

			return r.patchStatus(ctx, cache, orig)
		}

		return nil
	}

	if !rotationDue(cache) {
		return nil
	}

	if err := r.markIdle(ctx, cache); err != nil {
		return err
	}
	setRotationCondition(cache, metav1.ConditionFalse, ReasonRotating, "the password is being rotated")
	if err := r.patchStatus(ctx, cache, orig); err != nil {
		return err
	}
	logger.Info("password rotation started", "secret", cachepkg.AuthSecretName(cache))

	return r.copyPassword(ctx, cache)
}

// copyPassword runs the copy step and records it in the condition. A
// rejected copy is not an error here: completeRotation probes the next step.
func (r *CacheReconciler) copyPassword(ctx context.Context, cache *cachev1alpha1.Cache) error {
	body, err := cachepkg.RotationCopyPatch(cache)
	if err != nil {
		return err
	}
	if err := r.patchAuthSecret(ctx, cache, body); err != nil {
		if errors.Is(err, errPatchRejected) {
			return nil
		}
		return err
	}
	orig := cache.DeepCopy()
	setRotationCondition(cache, metav1.ConditionFalse, ReasonPasswordCopied, "the previous password is copied; the new one is next")

	return r.patchStatus(ctx, cache, orig)
}

// replacePassword writes the new password. A rejected replace after a
// confirmed copy means the replace already happened before a restart. A
// rejected replace without a confirmed copy means the Secret is not the
// operator's: both guarded patches failed on it.
func (r *CacheReconciler) replacePassword(ctx context.Context, cache *cachev1alpha1.Cache, copyConfirmed bool) error {
	body, err := cachepkg.RotationReplacePatch(cache, rand.Text())
	if err != nil {
		return err
	}
	err = r.patchAuthSecret(ctx, cache, body)
	switch {
	case err == nil, errors.Is(err, errPatchRejected) && copyConfirmed:
		return nil
	case errors.Is(err, errPatchRejected):
		return fmt.Errorf("the Secret %s does not carry the Cache label and the rotation step: %w",
			cachepkg.AuthSecretName(cache), err)
	default:
		return err
	}
}

// completeRotation runs after the ValkeyCluster apply. It writes the new
// password once the ValkeyCluster lists both keys, and removes the previous
// password once the ValkeyCluster lists one key again. It returns how long to
// wait before the next look.
func (r *CacheReconciler) completeRotation(ctx context.Context, cache *cachev1alpha1.Cache) (time.Duration, error) {
	logger := logf.FromContext(ctx)
	cond := rotationCondition(cache)
	if cond == nil {
		return 0, nil
	}
	if cache.Status.PasswordRotation == nil {
		cache.Status.PasswordRotation = &cachev1alpha1.PasswordRotationStatus{}
	}
	status := cache.Status.PasswordRotation

	switch {
	case cond.Status == metav1.ConditionFalse && (cond.Reason == ReasonRotating || cond.Reason == ReasonPasswordCopied):
		if cond.Reason == ReasonRotating {
			// The copy was not confirmed: a restart, or a rejected copy. The
			// copy is safe to repeat, and its result decides the probe.
			if err := r.copyPassword(ctx, cache); err != nil {
				return 0, err
			}
		}
		copyConfirmed := rotationCondition(cache).Reason == ReasonPasswordCopied
		if err := r.replacePassword(ctx, cache, copyConfirmed); err != nil {
			return 0, err
		}
		orig := cache.DeepCopy()
		now := r.now()
		period := r.previousValidFor(cache)
		status.LastRotatedAt = &metav1.Time{Time: now}
		status.PreviousValidUntil = &metav1.Time{Time: now.Add(period)}
		if spec := cache.Spec.PasswordRotation; spec != nil && spec.RequestedAt != nil {
			status.RequestedAt = spec.RequestedAt.DeepCopy()
		}
		setRotationCondition(cache, metav1.ConditionFalse, ReasonPreviousPasswordValid,
			"the new password is in place; the previous one stays valid until "+status.PreviousValidUntil.UTC().Format(time.RFC3339))
		logger.Info("password rotated", "secret", cachepkg.AuthSecretName(cache), "previousValidUntil", status.PreviousValidUntil.UTC())

		return max(period, time.Second), r.patchStatus(ctx, cache, orig)

	case cond.Status == metav1.ConditionTrue && status.PreviousValidUntil != nil:
		// A rejected remove means the key is already gone: only the
		// operator's own replace can have put the Secret in this state.
		body, err := cachepkg.RotationRemovePatch(cache)
		if err != nil {
			return 0, err
		}
		if err := r.patchAuthSecret(ctx, cache, body); err != nil && !errors.Is(err, errPatchRejected) {
			return 0, err
		}
		orig := cache.DeepCopy()
		status.PreviousValidUntil = nil
		setRotationCondition(cache, metav1.ConditionTrue, ReasonRotated, "the previous password is removed")
		logger.Info("previous password removed", "secret", cachepkg.AuthSecretName(cache))

		return 0, r.patchStatus(ctx, cache, orig)

	case cond.Status == metav1.ConditionFalse && status.PreviousValidUntil != nil:
		return max(status.PreviousValidUntil.Sub(r.now()), time.Second), nil
	}

	return 0, nil
}

// rotationFailed records a rotation error in the PasswordRotated condition
// and returns the error. The condition stays False, so the ValkeyCluster
// keeps both keys until someone looks.
func (r *CacheReconciler) rotationFailed(ctx context.Context, cache *cachev1alpha1.Cache, err error) error {
	orig := cache.DeepCopy()
	setRotationCondition(cache, metav1.ConditionFalse, ReasonRotationFailed, shortened(err.Error()))
	if statusErr := r.patchStatus(ctx, cache, orig); statusErr != nil {
		logf.FromContext(ctx).Error(statusErr, "cannot record the rotation failure in the Cache status")
	}

	return err
}
