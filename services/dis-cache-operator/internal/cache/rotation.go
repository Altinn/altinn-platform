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

	// RotationStepAnnotation records on the Secret how far a rotation got.
	// Every rotation patch tests it first. A failed test tells the operator
	// the state of the Secret without a read, and a Secret that someone else
	// made never gets rotated.
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
// previous password goes away and the Secret is idle again. It runs after the
// ValkeyCluster no longer lists the previous key.
func RotationRemovePatch(cache *cachev1alpha1.Cache) ([]byte, error) {
	ops := append(rotationGuard(cache, RotationStepReplaced),
		jsonPatchOp{Op: opRemove, Path: dataPath(AuthSecretPreviousPasswordKey)},
		jsonPatchOp{Op: opReplace, Path: rotationStepPath(), Value: RotationStepIdle},
	)

	return json.Marshal(ops)
}

// RotationMarkIdlePatch returns the merge patch that sets the rotation step
// annotation to idle. It is for Secrets the operator created before the
// annotation existed. The controller uses it only when no rotation is in
// progress, so the annotation is idle in both cases.
func RotationMarkIdlePatch() ([]byte, error) {
	return json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{RotationStepAnnotation: RotationStepIdle},
		},
	})
}

// PreviousPasswordInUse reports whether the ValkeyCluster must accept the
// previous password: from the start of a rotation until the previous
// password is removed. The controller keeps the PasswordRotated condition
// False during that time.
func PreviousPasswordInUse(cache *cachev1alpha1.Cache) bool {
	cond := meta.FindStatusCondition(cache.Status.Conditions, string(cachev1alpha1.ConditionPasswordRotated))

	return cond != nil && cond.Status == metav1.ConditionFalse
}

// PasswordKeys returns the Secret keys the app user accepts as passwords.
func PasswordKeys(cache *cachev1alpha1.Cache) []string {
	if PreviousPasswordInUse(cache) {
		return []string{AuthSecretPasswordKey, AuthSecretPreviousPasswordKey}
	}

	return []string{AuthSecretPasswordKey}
}
