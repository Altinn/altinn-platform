package controller

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	valkeyv1alpha1 "github.com/valkey-io/valkey-operator/api/v1alpha1"
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

// Reasons of the PasswordRotated condition. Rotating and PasswordStaged are
// the two steps before the new password moves into the password key; the
// reason records which Secret patch the operator has confirmed, so a restart
// resumes at the right step without a read of the Secret.
const (
	ReasonRotated               = "Rotated"
	ReasonRotating              = "Rotating"
	ReasonPasswordStaged        = cachepkg.ReasonPasswordStaged
	ReasonPreviousPasswordValid = cachepkg.ReasonPreviousPasswordValid
	ReasonRotationFailed        = "RotationFailed"

	// reasonPasswordCopied is the reason the operator before the staged key
	// used after it copied the password to the previous key. A Cache can
	// still carry it after an upgrade. The operator treats it as Rotating:
	// the stage is rejected, and findStep takes the Secret back to idle.
	reasonPasswordCopied = "PasswordCopied"

	// DefaultPreviousValidFor is the period the previous password stays valid
	// when neither the Cache nor the operator flag sets one.
	DefaultPreviousValidFor = 7 * 24 * time.Hour

	// scheduleRecheck is the longest time an idle Cache with a schedule waits
	// before the operator looks again. A rotation that is due earlier gets a
	// requeue at its own time.
	scheduleRecheck = 24 * time.Hour

	// aclApplyRequeue is how long the operator waits between two looks at the
	// ValkeyNodes while the staged password is not live on every node. The
	// ValkeyNode watch ends the wait early.
	aclApplyRequeue = 10 * time.Second

	// restageAfter is how long after the stage the operator waits for a node
	// that reports the ACL applied without a transition. Such a node gives no
	// proof that it loaded the staged password: it can have read the new file
	// on its first load, or it has not reloaded yet. The operator then stages
	// a fresh password. That is a new ACL change, so a working node reports a
	// new transition. The key "password" does not change while it waits.
	restageAfter = 5 * time.Minute
)

// errPatchRejected means the API server refused a rotation patch because one
// of its test operations failed: the step is not the expected one, or the
// Secret does not carry the Cache label.
var errPatchRejected = errors.New("the rotation patch was rejected")

// errForeignSecret means every guarded patch was rejected: the Secret with
// the auth Secret name is not the operator's. This is the one terminal error
// of a rotation.
var errForeignSecret = errors.New("the Secret does not carry the Cache label and the rotation step")

// errAdopted means a Secret from before the step annotation existed got its
// first annotation, idle. The rotation goes on in the next reconcile.
var errAdopted = errors.New("the Secret got its first rotation step annotation")

// errResetCopied means a Secret from the order before the staged key existed
// went back to idle. The rotation starts over in the next reconcile.
var errResetCopied = errors.New("the Secret from the previous rotation order went back to idle")

// errNodesPending means a Valkey node does not report its ACL applied, so the
// stage waits. The ValkeyNode watch and the requeue end the wait.
var errNodesPending = errors.New("a Valkey node does not report its ACL applied")

// intervalDays returns the schedule of the Cache in days, 0 when it is off.
// The API server sets the default, so nil only happens in memory.
func intervalDays(cache *cachev1alpha1.Cache) int32 {
	spec := cache.Spec.PasswordRotation
	if spec == nil || spec.IntervalDays == nil {
		return 0
	}

	return *spec.IntervalDays
}

// nextScheduledRotation returns when the schedule makes the next rotation
// due: intervalDays after the last rotation, or after the creation of the
// Cache when it never rotated. ok is false when the schedule is off.
func nextScheduledRotation(cache *cachev1alpha1.Cache) (at time.Time, ok bool) {
	days := intervalDays(cache)
	if days <= 0 {
		return time.Time{}, false
	}
	base := cache.CreationTimestamp.Time
	if status := cache.Status.PasswordRotation; status != nil && status.LastRotatedAt != nil {
		base = status.LastRotatedAt.Time
	}

	return base.Add(time.Duration(days) * 24 * time.Hour), true
}

// scheduleDue reports whether the schedule makes a rotation due now.
func (r *CacheReconciler) scheduleDue(cache *cachev1alpha1.Cache) bool {
	at, ok := nextScheduledRotation(cache)

	return ok && !r.now().Before(at)
}

// untilScheduledRotation returns how long an idle Cache waits for its next
// scheduled rotation, at most scheduleRecheck. It returns 0 when the
// schedule is off or a rotation is in progress; the rotation steps set their
// own requeue.
func (r *CacheReconciler) untilScheduledRotation(cache *cachev1alpha1.Cache) time.Duration {
	if cond := rotationCondition(cache); cond != nil && cond.Status == metav1.ConditionFalse {
		return 0
	}
	at, ok := nextScheduledRotation(cache)
	if !ok {
		return 0
	}

	return min(max(at.Sub(r.now()), time.Second), scheduleRecheck)
}

// requestDue reports whether the team asked for a rotation that the operator
// has not carried out yet.
func requestDue(cache *cachev1alpha1.Cache) bool {
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
	period := DefaultPreviousValidFor
	switch spec := cache.Spec.PasswordRotation; {
	case spec != nil && spec.PreviousValidFor != nil:
		period = spec.PreviousValidFor.Duration
	case r.PreviousValidFor > 0:
		period = r.PreviousValidFor
	}
	// With a schedule, the period is at most half of the interval, so the
	// old password is gone well before the next rotation. The API checks
	// this only for a period that the Cache sets itself.
	if days := intervalDays(cache); days > 0 {
		period = min(period, time.Duration(days)*12*time.Hour)
	}

	return period
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

func rotationStatus(cache *cachev1alpha1.Cache) *cachev1alpha1.PasswordRotationStatus {
	if cache.Status.PasswordRotation == nil {
		cache.Status.PasswordRotation = &cachev1alpha1.PasswordRotationStatus{}
	}

	return cache.Status.PasswordRotation
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

// probeStep reports whether the Secret is the operator's and at the given
// step. It changes nothing.
func (r *CacheReconciler) probeStep(ctx context.Context, cache *cachev1alpha1.Cache, step string) (bool, error) {
	probe, err := cachepkg.RotationStepProbe(cache, step)
	if err != nil {
		return false, err
	}
	switch err := r.patchAuthSecret(ctx, cache, probe); {
	case err == nil:
		return true, nil
	case errors.Is(err, errPatchRejected):
		return false, nil
	default:
		return false, err
	}
}

// beginRotation runs before the ValkeyCluster apply. It resets the rotation
// state when the Secret was created again, ends the period when it is over,
// and starts a rotation the team asked for. It writes the status before the
// first Secret patch, so the key list of the ValkeyCluster never names a key
// before the key exists.
func (r *CacheReconciler) beginRotation(ctx context.Context, cache *cachev1alpha1.Cache, secretCreated bool) error {
	logger := logf.FromContext(ctx)
	orig := cache.DeepCopy()
	cond := rotationCondition(cache)
	inProgress := cond != nil && cond.Status == metav1.ConditionFalse

	if secretCreated && inProgress {
		// The Secret came back with one password. The other keys are gone,
		// so the ValkeyCluster must list one key again.
		// The new Secret holds a new password, so the request is done too.
		logger.Info("the auth Secret was created again during a rotation; the rotation state is reset")
		setRotationCondition(cache, metav1.ConditionTrue, ReasonRotated, "the Secret was created again; the rotation was reset")
		status := rotationStatus(cache)
		status.StagedAt = nil
		status.PreviousValidUntil = nil
		status.LastRotatedAt = &metav1.Time{Time: r.now()}
		if spec := cache.Spec.PasswordRotation; spec != nil && spec.RequestedAt != nil {
			status.RequestedAt = spec.RequestedAt.DeepCopy()
		}

		return r.patchStatus(ctx, cache, orig)
	}

	if secretCreated && !inProgress {
		// A new Secret holds a new password, so the schedule counts from now.
		// This covers a new Cache and a Secret that someone deleted while
		// the Cache was idle.
		rotationStatus(cache).LastRotatedAt = &metav1.Time{Time: r.now()}
		if err := r.patchStatus(ctx, cache, orig); err != nil {
			return err
		}
		orig = cache.DeepCopy()
	}

	if inProgress && cond.Reason == ReasonRotationFailed && requestDue(cache) {
		// A newer request after the terminal failure starts over, for example
		// after someone removed the foreign Secret. The schedule does not: it
		// would try the foreign Secret again on every reconcile.
		inProgress = false
	}

	if inProgress {
		status := cache.Status.PasswordRotation
		if cond.Reason == ReasonPreviousPasswordValid && status != nil && status.PreviousValidUntil != nil &&
			!r.now().Before(status.PreviousValidUntil.Time) {
			// The period is over. The condition goes True first, so the apply
			// lists one key before the previous password is removed.
			setRotationCondition(cache, metav1.ConditionTrue, ReasonRotated, "the operator removes the previous password")

			return r.patchStatus(ctx, cache, orig)
		}

		return nil
	}

	requested := requestDue(cache)
	if !requested && !r.scheduleDue(cache) {
		return nil
	}

	message := "the operator rotates the password on request"
	if !requested {
		message = "the operator rotates the password on schedule"
	}
	setRotationCondition(cache, metav1.ConditionFalse, ReasonRotating, message)
	if err := r.patchStatus(ctx, cache, orig); err != nil {
		return err
	}
	logger.Info("password rotation started", "secret", cachepkg.AuthSecretName(cache), "requested", requested)

	// A Secret that is not idle gets its step found before the stage, so a
	// Secret that is not the operator's fails here and does not wait for the
	// nodes.
	idle, err := r.probeStep(ctx, cache, cachepkg.RotationStepIdle)
	if err != nil {
		return err
	}
	if !idle {
		err := r.findStep(ctx, cache)
		if errors.Is(err, errAdopted) || errors.Is(err, errResetCopied) {
			return nil
		}

		return err
	}
	if err := r.stagePassword(ctx, cache); err != nil && !errors.Is(err, errNodesPending) {
		return err
	}

	return nil
}

// stagePassword writes the new password under the next key and then records
// the step, with the time from before the patch, so the wait for the Valkey
// nodes covers the write. The status follows the patch: a listed key must
// exist. It stages only when every node reports its ACL applied, so a later
// transition belongs to the staged file. A rejected stage is not an error
// here: completeRotation probes the next step.
func (r *CacheReconciler) stagePassword(ctx context.Context, cache *cachev1alpha1.Cache) error {
	pending, _, err := r.nodesPendingACL(ctx, cache, nil, nil)
	if err != nil {
		return err
	}
	if pending != "" {
		message := "the operator waits for " + pending + " before it stages the new password"
		if cond := rotationCondition(cache); cond == nil || cond.Message != message {
			orig := cache.DeepCopy()
			setRotationCondition(cache, metav1.ConditionFalse, ReasonRotating, message)
			if err := r.patchStatus(ctx, cache, orig); err != nil {
				return err
			}
		}

		return errNodesPending
	}
	body, err := cachepkg.RotationStagePatch(cache, rand.Text())
	if err != nil {
		return err
	}
	stagedAt := r.now()
	if err := r.patchAuthSecret(ctx, cache, body); err != nil {
		if errors.Is(err, errPatchRejected) {
			return nil
		}
		return err
	}

	return r.recordStage(ctx, cache, stagedAt)
}

// recordStage writes the staged state to the status: the key list names all
// three keys from here on, and the wait for the nodes starts.
func (r *CacheReconciler) recordStage(ctx context.Context, cache *cachev1alpha1.Cache, stagedAt time.Time) error {
	return r.recordStageWith(ctx, cache, stagedAt, "the new password is staged; the operator waits until every Valkey node accepts it")
}

func (r *CacheReconciler) recordStageWith(ctx context.Context, cache *cachev1alpha1.Cache, stagedAt time.Time, message string) error {
	orig := cache.DeepCopy()
	rotationStatus(cache).StagedAt = &metav1.Time{Time: stagedAt}
	setRotationCondition(cache, metav1.ConditionFalse, ReasonPasswordStaged, message)

	return r.patchStatus(ctx, cache, orig)
}

// restagePassword writes a fresh password under the next key and moves the
// stage time.
//
// Known limit: a fresh stage does not change the ValkeyCluster generation,
// so the processed check still holds from the first stage. A node whose
// mount stayed stale for the first value longer than restageAfter can flip
// in the seconds before the valkey-operator builds the fresh ACL, and that
// flip counts. The ACL revision in the valkey-operator status closes this;
// until then the window stays open on purpose. The status goes first: a transition before the new stage time
// must not count for the fresh value, also when the patch or a later status
// write fails. A failed patch then only delays the rotation. The stage time
// moves the staged-time annotation on the ValkeyCluster in the next
// reconcile, and that change makes the valkey-operator build the new ACL. A
// rejected patch means the Secret is not at the staged step any more; findStep
// finds the step.
func (r *CacheReconciler) restagePassword(ctx context.Context, cache *cachev1alpha1.Cache, node string) error {
	body, err := cachepkg.RotationRestagePatch(cache, rand.Text())
	if err != nil {
		return err
	}
	if err := r.recordStageWith(ctx, cache, r.now(),
		"the operator staged the new password again, because "+node+" reported no new ACL change after the stage"); err != nil {
		return err
	}
	switch err := r.patchAuthSecret(ctx, cache, body); {
	case errors.Is(err, errPatchRejected):
		return r.findStep(ctx, cache)
	case err != nil:
		return err
	}
	logf.FromContext(ctx).Info("password staged again", "secret", cachepkg.AuthSecretName(cache), "node", node)

	return nil
}

// findStep runs after a rejected stage. It tells, with probes only, which
// step the Secret is at: staged or replaced from an earlier reconcile whose
// status writes were lost, copied from the order before the staged key, or
// from before the annotation existed. When no probe matches, the Secret is
// not the operator's.
func (r *CacheReconciler) findStep(ctx context.Context, cache *cachev1alpha1.Cache) error {
	logger := logf.FromContext(ctx)

	idle, err := r.probeStep(ctx, cache, cachepkg.RotationStepIdle)
	if err != nil {
		return err
	}
	if idle {
		// Someone took the Secret back to idle, for example with a new
		// password. The rotation starts over from the stage.
		orig := cache.DeepCopy()
		rotationStatus(cache).StagedAt = nil
		setRotationCondition(cache, metav1.ConditionFalse, ReasonRotating, "the Secret is idle again; the operator stages the new password again")

		return r.patchStatus(ctx, cache, orig)
	}

	staged, err := r.probeStep(ctx, cache, cachepkg.RotationStepStaged)
	if err != nil {
		return err
	}
	if staged {
		// The stage time is lost with the status. The wait starts over from
		// now, which is later than the write, so it is safe.
		return r.recordStage(ctx, cache, r.now())
	}

	replaced, err := r.probeStep(ctx, cache, cachepkg.RotationStepReplaced)
	if err != nil {
		return err
	}
	if replaced {
		return r.recordPromotion(ctx, cache)
	}

	reset, err := cachepkg.RotationResetCopiedPatch(cache)
	if err != nil {
		return err
	}
	switch err := r.patchAuthSecret(ctx, cache, reset); {
	case err == nil:
		logger.Info("the auth Secret from the previous rotation order went back to idle", "secret", cachepkg.AuthSecretName(cache))

		return errResetCopied
	case !errors.Is(err, errPatchRejected):
		return err
	}

	// A Secret of this Cache from before the annotation existed gets the
	// idle step. The first patch adds the one annotation; it fails when the
	// Secret has no annotations map, and the second patch adds the map.
	for _, build := range []func(*cachev1alpha1.Cache) ([]byte, error){cachepkg.RotationAdoptPatch, cachepkg.RotationAdoptMapPatch} {
		adopt, err := build(cache)
		if err != nil {
			return err
		}
		switch err := r.patchAuthSecret(ctx, cache, adopt); {
		case err == nil:
			return errAdopted
		case !errors.Is(err, errPatchRejected):
			return err
		}
	}

	return fmt.Errorf("%w: %s", errForeignSecret, cachepkg.AuthSecretName(cache))
}

// nodesPendingACL names what the rotation waits for, or "" when nothing is
// pending. The valkey-operator sets the ACLApplied condition of a ValkeyNode
// to True only after it has reloaded the ACL file and verified the password
// hashes on that node, and to False while the mounted file is stale.
//
// Before the stage (stagedAt nil) every node must report True, with a
// transition before this second, so a later transition belongs to the staged
// file. After the stage, the valkey-operator must first have processed the
// ValkeyCluster generation that lists the new key, which its conditions show
// through the observed generation. Then every node counts only with True and
// a transition at or after the stage. Without a node there is nothing that
// accepts a password yet.
//
// unproven names the first node that reports True without such a
// transition, when every node reports True. That node gives no proof, and the
// caller stages again after restageAfter.
func (r *CacheReconciler) nodesPendingACL(ctx context.Context, cache *cachev1alpha1.Cache, stagedAt *metav1.Time, cluster *valkeyv1alpha1.ValkeyCluster) (pending, unproven string, err error) {
	if stagedAt != nil && !clusterProcessed(cluster) {
		return "the valkey-operator to read the new key list", "", nil
	}
	var nodes valkeyv1alpha1.ValkeyNodeList
	if err := r.List(ctx, &nodes, client.InNamespace(cache.Namespace),
		client.MatchingLabels{cachepkg.ValkeyClusterLabel: cachepkg.ValkeyClusterName(cache)}); err != nil {
		return "", "", fmt.Errorf("list ValkeyNodes: %w", err)
	}
	if len(nodes.Items) == 0 {
		return "a Valkey node", "", nil
	}
	now := r.now()
	for i := range nodes.Items {
		node := &nodes.Items[i]
		cond := meta.FindStatusCondition(node.Status.Conditions, valkeyv1alpha1.ValkeyNodeConditionACLApplied)
		if cond == nil || cond.Status != metav1.ConditionTrue {
			return "Valkey node " + node.Name, "", nil
		}
		if stagedAt == nil && !cond.LastTransitionTime.Time.Before(now) {
			return "Valkey node " + node.Name, "", nil
		}
		if stagedAt != nil && cond.LastTransitionTime.Before(stagedAt) && unproven == "" {
			unproven = "Valkey node " + node.Name
		}
	}
	if unproven != "" {
		return unproven, unproven, nil
	}

	return "", "", nil
}

// clusterProcessed reports whether the valkey-operator has reconciled the
// current generation of the ValkeyCluster: it writes the generation it saw
// into every condition it sets.
func clusterProcessed(cluster *valkeyv1alpha1.ValkeyCluster) bool {
	if cluster == nil {
		return false
	}
	for i := range cluster.Status.Conditions {
		if cluster.Status.Conditions[i].ObservedGeneration >= cluster.Generation {
			return true
		}
	}

	return false
}

// promotePassword moves the staged password into the password key. A
// rejected promote is either a promote that already happened before a
// restart, which the probe confirms, or a Secret at another step, which
// findStep sorts out.
func (r *CacheReconciler) promotePassword(ctx context.Context, cache *cachev1alpha1.Cache) error {
	body, err := cachepkg.RotationPromotePatch(cache)
	if err != nil {
		return err
	}
	switch err := r.patchAuthSecret(ctx, cache, body); {
	case err == nil:
		return r.recordPromotion(ctx, cache)
	case !errors.Is(err, errPatchRejected):
		return err
	}
	replaced, err := r.probeStep(ctx, cache, cachepkg.RotationStepReplaced)
	if err != nil {
		return err
	}
	if replaced {
		return r.recordPromotion(ctx, cache)
	}

	return r.findStep(ctx, cache)
}

// recordPromotion writes the rotation time and the period to the status: the
// new password is in place, the previous one stays valid until the period
// ends.
func (r *CacheReconciler) recordPromotion(ctx context.Context, cache *cachev1alpha1.Cache) error {
	orig := cache.DeepCopy()
	status := rotationStatus(cache)
	now := r.now()
	period := r.previousValidFor(cache)
	status.StagedAt = nil
	status.LastRotatedAt = &metav1.Time{Time: now}
	status.PreviousValidUntil = &metav1.Time{Time: now.Add(period)}
	if spec := cache.Spec.PasswordRotation; spec != nil && spec.RequestedAt != nil {
		status.RequestedAt = spec.RequestedAt.DeepCopy()
	}
	setRotationCondition(cache, metav1.ConditionFalse, ReasonPreviousPasswordValid,
		"the new password is in place; the previous one stays valid until "+status.PreviousValidUntil.UTC().Format(time.RFC3339))
	logf.FromContext(ctx).Info("password rotated", "secret", cachepkg.AuthSecretName(cache), "previousValidUntil", status.PreviousValidUntil.UTC())

	return r.patchStatus(ctx, cache, orig)
}

// completeRotation runs after the ValkeyCluster apply. It stages the new
// password when the stage was not confirmed, promotes it once every Valkey
// node accepts it, and removes the old passwords once the ValkeyCluster lists
// one key again. It returns how long to wait before the next look.
func (r *CacheReconciler) completeRotation(ctx context.Context, cache *cachev1alpha1.Cache, cluster *valkeyv1alpha1.ValkeyCluster) (time.Duration, error) {
	logger := logf.FromContext(ctx)
	cond := rotationCondition(cache)
	if cond == nil {
		return 0, nil
	}
	status := rotationStatus(cache)

	switch {
	case cond.Status == metav1.ConditionFalse && (cond.Reason == ReasonRotating || cond.Reason == reasonPasswordCopied):
		// The stage was not confirmed: a restart, a wait for the nodes, or a
		// rejected stage. The stage is safe to repeat, and its result decides
		// the probes.
		err := r.stagePassword(ctx, cache)
		switch {
		case errors.Is(err, errNodesPending):
			return aclApplyRequeue, nil
		case err != nil:
			return 0, err
		}
		if rotationCondition(cache).Reason == ReasonPasswordStaged {
			// The stage is confirmed now, but this apply listed one key. The
			// next reconcile lists all keys, and the wait starts.
			return time.Second, nil
		}
		err = r.findStep(ctx, cache)
		switch {
		case errors.Is(err, errAdopted):
			logger.Info("the auth Secret got its rotation step annotation", "secret", cachepkg.AuthSecretName(cache))

			return time.Second, nil
		case errors.Is(err, errResetCopied):
			return time.Second, nil
		case err != nil:
			return 0, err
		}

		return time.Second, nil

	case cond.Status == metav1.ConditionFalse && cond.Reason == ReasonPasswordStaged:
		return r.waitForStagedPassword(ctx, cache, cluster)

	case cond.Status == metav1.ConditionTrue && status.PreviousValidUntil != nil:
		// A rejected remove means the keys are already gone: only the
		// operator's own promote can have put the Secret in this state.
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

	case cond.Status == metav1.ConditionFalse && cond.Reason == ReasonPreviousPasswordValid && status.PreviousValidUntil != nil:
		return max(status.PreviousValidUntil.Sub(r.now()), time.Second), nil
	}

	return 0, nil
}

// waitForStagedPassword runs while a new password is staged. It promotes the
// password once the valkey-operator has processed the new key list and every
// Valkey node proves it with a transition after the stage. It stages a fresh
// password when a node gives no proof within restageAfter.
func (r *CacheReconciler) waitForStagedPassword(ctx context.Context, cache *cachev1alpha1.Cache, cluster *valkeyv1alpha1.ValkeyCluster) (time.Duration, error) {
	cond := rotationCondition(cache)
	status := rotationStatus(cache)
	if status.StagedAt == nil {
		// The stage time is lost. The wait starts over from now, which is
		// later than the write, so it is safe.
		return aclApplyRequeue, r.recordStage(ctx, cache, r.now())
	}
	pending, unproven, err := r.nodesPendingACL(ctx, cache, status.StagedAt, cluster)
	if err != nil {
		return 0, err
	}
	if unproven != "" && !r.now().Before(status.StagedAt.Add(restageAfter)) {
		err := r.restagePassword(ctx, cache, unproven)
		switch {
		case errors.Is(err, errAdopted), errors.Is(err, errResetCopied):
			return time.Second, nil
		case err != nil:
			return 0, err
		}

		return aclApplyRequeue, nil
	}
	if pending != "" {
		message := "the new password is staged; " + pending + " does not report it yet"
		if message != cond.Message {
			orig := cache.DeepCopy()
			setRotationCondition(cache, metav1.ConditionFalse, ReasonPasswordStaged, message)
			if err := r.patchStatus(ctx, cache, orig); err != nil {
				return 0, err
			}
		}

		return aclApplyRequeue, nil
	}
	err = r.promotePassword(ctx, cache)
	switch {
	case errors.Is(err, errAdopted), errors.Is(err, errResetCopied):
		return time.Second, nil
	case err != nil:
		return 0, err
	}

	// The next reconcile writes the rotation time on the ValkeyCluster.
	return time.Second, nil
}

// rotationFailed records a rotation error in the PasswordRotated condition
// and returns the error. The reason records the confirmed step, so a passing
// error keeps the reason and only sets the message: the next reconcile
// resumes at the same step. A foreign Secret is terminal: the condition gets
// the reason RotationFailed, the request counts as handled, and the
// ValkeyCluster lists one key. A later request starts over.
func (r *CacheReconciler) rotationFailed(ctx context.Context, cache *cachev1alpha1.Cache, err error) error {
	orig := cache.DeepCopy()
	cond := rotationCondition(cache)
	switch {
	case errors.Is(err, errForeignSecret):
		setRotationCondition(cache, metav1.ConditionFalse, ReasonRotationFailed, shortened(err.Error()))
		status := rotationStatus(cache)
		status.StagedAt = nil
		if spec := cache.Spec.PasswordRotation; spec != nil && spec.RequestedAt != nil {
			status.RequestedAt = spec.RequestedAt.DeepCopy()
		}
	case cond != nil:
		setRotationCondition(cache, cond.Status, cond.Reason, shortened(err.Error()))
	default:
		return err
	}
	if statusErr := r.patchStatus(ctx, cache, orig); statusErr != nil {
		logf.FromContext(ctx).Error(statusErr, "cannot record the rotation failure in the Cache status")
	}

	return err
}
