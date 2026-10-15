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
	// AuthSecretNextPasswordKey holds the new password while Valkey loads it.
	// Applications never read this key. It moves to the password key once
	// every Valkey node accepts it, so a pod that starts at any moment reads
	// a password that Valkey accepts.
	AuthSecretNextPasswordKey = "password-next"

	// AuthSecretPreviousPasswordKey holds the previous password during a
	// rotation period. Both passwords are valid until the period ends.
	AuthSecretPreviousPasswordKey = "password-previous"

	// RotationStepAnnotation records on the Secret which rotation step is
	// done. Every rotation patch tests it first, so a step out of order or a
	// Secret that someone else made stops the patch. A failed test is one
	// error without a reason. The operator never reads the Secret. So the
	// controller finds the step with probes: patches with tests only.
	RotationStepAnnotation = "cache.dis.altinn.cloud/rotation-step"

	// RotationStepIdle means no rotation is in progress.
	RotationStepIdle = "idle"
	// RotationStepStaged means the next key holds the new password and the
	// password key still holds the current one.
	RotationStepStaged = "staged"
	// RotationStepReplaced means the new password is in place and the
	// previous one is still valid.
	RotationStepReplaced = "replaced"
	// RotationStepCopied is the first step of the order before the staged
	// key existed: the previous key holds a copy of the password. The
	// controller resets such a Secret to idle and starts over.
	RotationStepCopied = "copied"
)

// The RFC 6902 operations the rotation patches use.
const (
	opTest    = "test"
	opAdd     = "add"
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

// RotationStagePatch returns the JSON patch for the first rotation step: the
// current password is copied to the previous key, and the new password is
// written under the next key. The API server copies the value; the operator
// never reads it. The password key does not change, so every application
// keeps a password that Valkey accepts. From this step until the removal the
// three keys exist and the ValkeyCluster lists all three, so the set of
// accepted passwords changes only here and at the removal. The patch fails
// when the Secret is not idle or does not belong to this Cache, so a retry
// after a crash never writes a second new password.
func RotationStagePatch(cache *cachev1alpha1.Cache, password string) ([]byte, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(password))
	ops := append(rotationGuard(cache, RotationStepIdle),
		jsonPatchOp{Op: opCopy, From: dataPath(AuthSecretPasswordKey), Path: dataPath(AuthSecretPreviousPasswordKey)},
		jsonPatchOp{Op: opAdd, Path: dataPath(AuthSecretNextPasswordKey), Value: encoded},
		jsonPatchOp{Op: opReplace, Path: rotationStepPath(), Value: RotationStepStaged},
	)

	return json.Marshal(ops)
}

// RotationPromotePatch returns the JSON patch for the second rotation step,
// after every Valkey node accepts the staged password: the staged password
// is copied to the password key. The API server copies the value; the
// operator never reads it. The previous and the next key keep their values,
// so the set of accepted passwords does not change.
func RotationPromotePatch(cache *cachev1alpha1.Cache) ([]byte, error) {
	ops := append(rotationGuard(cache, RotationStepStaged),
		jsonPatchOp{Op: opCopy, From: dataPath(AuthSecretNextPasswordKey), Path: dataPath(AuthSecretPasswordKey)},
		jsonPatchOp{Op: opReplace, Path: rotationStepPath(), Value: RotationStepReplaced},
	)

	return json.Marshal(ops)
}

// RotationRestagePatch returns the JSON patch that writes a fresh password
// under the next key while the step stays staged. The operator uses it when a
// Valkey node gives no proof that it loaded the staged password: a new value
// is a new ACL change, so a working node reports a new transition. Nothing
// reads the next key, so the earlier staged value is not in use.
func RotationRestagePatch(cache *cachev1alpha1.Cache, password string) ([]byte, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(password))
	ops := append(rotationGuard(cache, RotationStepStaged),
		jsonPatchOp{Op: opReplace, Path: dataPath(AuthSecretNextPasswordKey), Value: encoded},
	)

	return json.Marshal(ops)
}

// RotationRemovePatch returns the JSON patch for the last rotation step: the
// previous and the next password are removed and the Secret is idle again.
// The controller applies it after the ValkeyCluster lists the password key
// only. A rotation that the operator before the staged key started has no
// next key at this step. The add before the remove makes the remove work in
// both cases: add creates the key or replaces its value.
func RotationRemovePatch(cache *cachev1alpha1.Cache) ([]byte, error) {
	ops := append(rotationGuard(cache, RotationStepReplaced),
		jsonPatchOp{Op: opRemove, Path: dataPath(AuthSecretPreviousPasswordKey)},
		jsonPatchOp{Op: opAdd, Path: dataPath(AuthSecretNextPasswordKey), Value: ""},
		jsonPatchOp{Op: opRemove, Path: dataPath(AuthSecretNextPasswordKey)},
		jsonPatchOp{Op: opReplace, Path: rotationStepPath(), Value: RotationStepIdle},
	)

	return json.Marshal(ops)
}

// RotationStepProbe returns a JSON patch with tests only: the Secret belongs
// to this Cache and the rotation is at the given step. It changes nothing.
// The controller uses it after a rejected patch, to tell a step that already
// happened from a Secret that is not the operator's.
func RotationStepProbe(cache *cachev1alpha1.Cache, step string) ([]byte, error) {
	return json.Marshal(rotationGuard(cache, step))
}

// RotationResetCopiedPatch returns the JSON patch that takes a Secret from
// the order before the staged key existed back to idle. At the copied step
// the previous key holds a copy of the password that is still in use, so the
// copy can go. The rotation then starts over with the staged key.
func RotationResetCopiedPatch(cache *cachev1alpha1.Cache) ([]byte, error) {
	ops := append(rotationGuard(cache, RotationStepCopied),
		jsonPatchOp{Op: opRemove, Path: dataPath(AuthSecretPreviousPasswordKey)},
		jsonPatchOp{Op: opReplace, Path: rotationStepPath(), Value: RotationStepIdle},
	)

	return json.Marshal(ops)
}

// RotationAdoptPatch returns the JSON patch that gives a Secret from before
// the annotation existed its first step annotation, idle. It tests the Cache
// label and adds the one annotation, so other annotations stay. The add
// fails when the Secret has no annotations map at all; the controller then
// uses RotationAdoptMapPatch. The controller probes for every known step
// before it adopts, so the add never replaces a step that is in use.
func RotationAdoptPatch(cache *cachev1alpha1.Cache) ([]byte, error) {
	ops := []jsonPatchOp{
		{Op: opTest, Path: labelPath(CacheNameLabel), Value: cache.Name},
		{Op: opAdd, Path: rotationStepPath(), Value: RotationStepIdle},
	}

	return json.Marshal(ops)
}

// RotationAdoptMapPatch returns the JSON patch for a Secret of this Cache
// that has no annotations map: it adds the map with the idle step.
func RotationAdoptMapPatch(cache *cachev1alpha1.Cache) ([]byte, error) {
	ops := []jsonPatchOp{
		{Op: opTest, Path: labelPath(CacheNameLabel), Value: cache.Name},
		{Op: opAdd, Path: "/metadata/annotations", Value: map[string]string{RotationStepAnnotation: RotationStepIdle}},
	}

	return json.Marshal(ops)
}

// Reasons of the PasswordRotated condition that decide which Secret keys the
// ValkeyCluster lists. The controller sets them.
const (
	// ReasonPasswordStaged means the next key holds the new password and
	// the operator waits until every Valkey node accepts it.
	ReasonPasswordStaged = "PasswordStaged"
	// ReasonPreviousPasswordValid means the new password is in place and the
	// previous one is still valid.
	ReasonPreviousPasswordValid = "PreviousPasswordValid"
)

// PasswordKeys returns the Secret keys the app user accepts as passwords:
// the password key alone, all three keys while a password is staged, and the
// password and the previous key during the period. A listed key that does
// not exist makes the valkey-operator stop with an error, so a key is listed
// only while it is sure to exist. After the promote the password key and the
// next key hold the same value, so the period lists the same passwords as the
// stage did, and the ACL does not change. A rotation that the operator before
// the staged key started has no next key during the period, so the period
// does not list it.
func PasswordKeys(cache *cachev1alpha1.Cache) []string {
	cond := meta.FindStatusCondition(cache.Status.Conditions, string(cachev1alpha1.ConditionPasswordRotated))
	if cond == nil || cond.Status != metav1.ConditionFalse {
		return []string{AuthSecretPasswordKey}
	}
	switch cond.Reason {
	case ReasonPasswordStaged:
		return []string{AuthSecretPasswordKey, AuthSecretPreviousPasswordKey, AuthSecretNextPasswordKey}
	case ReasonPreviousPasswordValid:
		return []string{AuthSecretPasswordKey, AuthSecretPreviousPasswordKey}
	}

	return []string{AuthSecretPasswordKey}
}
