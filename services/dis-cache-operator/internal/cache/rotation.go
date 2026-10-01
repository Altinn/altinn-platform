package cache

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
)

const (
	// AuthSecretPreviousPasswordKey holds the previous password during a
	// rotation period. Both passwords are valid until the period ends.
	AuthSecretPreviousPasswordKey = "password-previous"

	// RotationStepAnnotation records on the Secret which rotation step is
	// done. Every rotation patch tests it first, so a step out of order or a
	// Secret that someone else made stops the patch. A failed test is one
	// error without a reason, and the operator never reads the Secret, so the
	// controller finds the step by trying the next guarded patch, and last a
	// probe with tests only.
	RotationStepAnnotation = "cache.dis.altinn.cloud/rotation-step"

	// RotationStepIdle means no rotation is in progress.
	RotationStepIdle = "idle"
	// RotationStepCopied means the previous key holds a copy of the password
	// and the new password is not written yet.
	RotationStepCopied = "copied"
	// RotationStepReplaced means the new password is in place and the
	// previous one is still valid.
	RotationStepReplaced = "replaced"
)

// The RFC 6902 operations the rotation patches use.
const (
	opTest    = "test"
	opCopy    = "copy"
	opReplace = "replace"
	opRemove  = "remove"
)

// jsonPatchOp is one operation of an RFC 6902 JSON patch.
type jsonPatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	From  string `json:"from,omitempty"`
	Value any    `json:"value,omitempty"`
}

// escapeJSONPointer escapes a map key for use in a JSON pointer.
func escapeJSONPointer(key string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(key)
}

func labelPath(key string) string {
	return "/metadata/labels/" + escapeJSONPointer(key)
}

// rotationStepPath is the JSON pointer of the rotation step annotation.
func rotationStepPath() string {
	return "/metadata/annotations/" + escapeJSONPointer(RotationStepAnnotation)
}

func dataPath(key string) string {
	return "/data/" + escapeJSONPointer(key)
}

// rotationGuard returns the test operations every rotation patch starts with:
// the Secret belongs to this Cache, and the rotation is at the expected step.
// The API server rejects the whole patch when a test fails.
func rotationGuard(cache *cachev1alpha1.Cache, step string) []jsonPatchOp {
	return []jsonPatchOp{
		{Op: opTest, Path: labelPath(CacheNameLabel), Value: cache.Name},
		{Op: opTest, Path: rotationStepPath(), Value: step},
	}
}

// RotationCopyPatch returns the JSON patch for the first rotation step: the
// current password is copied to the previous key. The API server copies the
// value; the operator never reads it. The patch fails when the Secret is not
// idle or does not belong to this Cache.
func RotationCopyPatch(cache *cachev1alpha1.Cache) ([]byte, error) {
	ops := append(rotationGuard(cache, RotationStepIdle),
		jsonPatchOp{Op: opCopy, From: dataPath(AuthSecretPasswordKey), Path: dataPath(AuthSecretPreviousPasswordKey)},
		jsonPatchOp{Op: opReplace, Path: rotationStepPath(), Value: RotationStepCopied},
	)

	return json.Marshal(ops)
}

// RotationReplacePatch returns the JSON patch for the second rotation step:
// the new password replaces the current one. The patch fails unless the copy
// step is done, so a retry after a crash never writes a second new password.
func RotationReplacePatch(cache *cachev1alpha1.Cache, password string) ([]byte, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(password))
	ops := append(rotationGuard(cache, RotationStepCopied),
		jsonPatchOp{Op: opReplace, Path: dataPath(AuthSecretPasswordKey), Value: encoded},
		jsonPatchOp{Op: opReplace, Path: rotationStepPath(), Value: RotationStepReplaced},
	)

	return json.Marshal(ops)
}

// RotationRemovePatch returns the JSON patch for the last rotation step: the
// previous password is removed and the Secret is idle again. The controller
// applies it after the ValkeyCluster no longer lists the previous key.
func RotationRemovePatch(cache *cachev1alpha1.Cache) ([]byte, error) {
	ops := append(rotationGuard(cache, RotationStepReplaced),
		jsonPatchOp{Op: opRemove, Path: dataPath(AuthSecretPreviousPasswordKey)},
		jsonPatchOp{Op: opReplace, Path: rotationStepPath(), Value: RotationStepIdle},
	)

	return json.Marshal(ops)
}

// RotationReplacedProbe returns a JSON patch with tests only: the Secret
// belongs to this Cache and the step is replaced. It changes nothing. The
// controller uses it after a rejected copy and a rejected replace, to tell a
// replace that already happened from a Secret that is not the operator's.
func RotationReplacedProbe(cache *cachev1alpha1.Cache) ([]byte, error) {
	return json.Marshal(rotationGuard(cache, RotationStepReplaced))
}

// RotationAdoptPatch returns the JSON patch that gives a Secret from before
// the annotation existed its first step annotation, idle. It tests the Cache
// label and then adds the whole annotations map, so it applies only to a
// Secret of this Cache that has no annotations yet: a Secret with a step
// annotation fails the add, because the controller probes for its step before
// it tries to adopt it.
func RotationAdoptPatch(cache *cachev1alpha1.Cache) ([]byte, error) {
	ops := []jsonPatchOp{
		{Op: opTest, Path: labelPath(CacheNameLabel), Value: cache.Name},
		{Op: "add", Path: "/metadata/annotations", Value: map[string]string{RotationStepAnnotation: RotationStepIdle}},
	}

	return json.Marshal(ops)
}

// Reasons of the PasswordRotated condition that mean the previous password
// key exists in the Secret. The controller sets them.
const (
	// ReasonPasswordCopied means the previous key holds a copy of the
	// password and the new password is not written yet.
	ReasonPasswordCopied = "PasswordCopied"
	// ReasonPreviousPasswordValid means the new password is in place and the
	// previous one is still valid.
	ReasonPreviousPasswordValid = "PreviousPasswordValid"
)

// PreviousPasswordInUse reports whether the ValkeyCluster must accept the
// previous password: from the confirmed copy until the previous password is
// removed. Before the copy is confirmed, and after a terminal failure, the
// previous key may not exist, and a listed key that does not exist makes the
// valkey-operator stop with an error.
func PreviousPasswordInUse(cache *cachev1alpha1.Cache) bool {
	cond := meta.FindStatusCondition(cache.Status.Conditions, string(cachev1alpha1.ConditionPasswordRotated))
	if cond == nil || cond.Status != metav1.ConditionFalse {
		return false
	}

	return cond.Reason == ReasonPasswordCopied || cond.Reason == ReasonPreviousPasswordValid
}

// PasswordKeys returns the Secret keys the app user accepts as passwords.
func PasswordKeys(cache *cachev1alpha1.Cache) []string {
	if PreviousPasswordInUse(cache) {
		return []string{AuthSecretPasswordKey, AuthSecretPreviousPasswordKey}
	}

	return []string{AuthSecretPasswordKey}
}
