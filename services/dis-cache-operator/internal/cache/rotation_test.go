package cache

import (
	"encoding/base64"
	"encoding/json"
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
)

const (
	labelPathWant      = "/metadata/labels/cache.dis.altinn.cloud~1cache"
	annotationPathWant = "/metadata/annotations/cache.dis.altinn.cloud~1rotation-step"
)

// decodePatch turns a JSON patch document into its operations.
func decodePatch(t *testing.T, body []byte, err error) []map[string]any {
	t.Helper()
	if err != nil {
		t.Fatalf("build patch: %v", err)
	}
	var ops []map[string]any
	if err := json.Unmarshal(body, &ops); err != nil {
		t.Fatalf("decode patch %s: %v", body, err)
	}

	return ops
}

// expectGuard checks the two test operations that start every rotation patch.
func expectGuard(t *testing.T, ops []map[string]any, step string) {
	t.Helper()
	if len(ops) < 2 {
		t.Fatalf("patch has %d operations, want the two guards first", len(ops))
	}
	if ops[0]["op"] != "test" || ops[0]["path"] != labelPathWant || ops[0]["value"] != "app-one-cache" {
		t.Errorf("guard 0: want a test on the cache label, got %v", ops[0])
	}
	if ops[1]["op"] != "test" || ops[1]["path"] != annotationPathWant || ops[1]["value"] != step {
		t.Errorf("guard 1: want a test on the rotation step %q, got %v", step, ops[1])
	}
}

func TestRotationStagePatch(t *testing.T) {
	t.Parallel()

	body, err := RotationStagePatch(newTestCache(nil), "new-password-456")
	ops := decodePatch(t, body, err)
	expectGuard(t, ops, RotationStepIdle)
	if len(ops) != 5 {
		t.Fatalf("want 5 operations, got %d: %v", len(ops), ops)
	}
	if ops[2]["op"] != "copy" || ops[2]["from"] != "/data/password" || ops[2]["path"] != "/data/password-previous" {
		t.Errorf("want the password copied to the previous key, got %v", ops[2])
	}
	if _, hasValue := ops[2]["value"]; hasValue {
		t.Errorf("the copy must carry no value: the operator never sees the password, got %v", ops[2])
	}
	want := base64.StdEncoding.EncodeToString([]byte("new-password-456"))
	if ops[3]["op"] != "add" || ops[3]["path"] != "/data/password-next" || ops[3]["value"] != want {
		t.Errorf("want the new password added under the next key, got %v", ops[3])
	}
	if ops[4]["op"] != "replace" || ops[4]["path"] != annotationPathWant || ops[4]["value"] != RotationStepStaged {
		t.Errorf("want the step set to staged, got %v", ops[4])
	}
}

func TestRotationPromotePatch(t *testing.T) {
	t.Parallel()

	body, err := RotationPromotePatch(newTestCache(nil))
	ops := decodePatch(t, body, err)
	expectGuard(t, ops, RotationStepStaged)
	if len(ops) != 4 {
		t.Fatalf("want 4 operations, got %d: %v", len(ops), ops)
	}
	if ops[2]["op"] != "copy" || ops[2]["from"] != "/data/password-next" || ops[2]["path"] != "/data/password" {
		t.Errorf("want the staged password copied to the password key, got %v", ops[2])
	}
	if _, hasValue := ops[2]["value"]; hasValue {
		t.Errorf("the copy must carry no value: the operator never sees the password, got %v", ops[2])
	}
	if ops[3]["op"] != "replace" || ops[3]["path"] != annotationPathWant || ops[3]["value"] != RotationStepReplaced {
		t.Errorf("want the step set to replaced, got %v", ops[3])
	}
}

func TestRotationRestagePatch(t *testing.T) {
	t.Parallel()

	body, err := RotationRestagePatch(newTestCache(nil), "fresh-password-789")
	ops := decodePatch(t, body, err)
	expectGuard(t, ops, RotationStepStaged)
	if len(ops) != 3 {
		t.Fatalf("want 3 operations, got %d: %v", len(ops), ops)
	}
	want := base64.StdEncoding.EncodeToString([]byte("fresh-password-789"))
	if ops[2]["op"] != "replace" || ops[2]["path"] != "/data/password-next" || ops[2]["value"] != want {
		t.Errorf("want only the next key replaced, the step unchanged, got %v", ops[2])
	}
}

func TestRotationRemovePatch(t *testing.T) {
	t.Parallel()

	body, err := RotationRemovePatch(newTestCache(nil))
	ops := decodePatch(t, body, err)
	expectGuard(t, ops, RotationStepReplaced)
	if len(ops) != 6 {
		t.Fatalf("want 6 operations, got %d: %v", len(ops), ops)
	}
	if ops[2]["op"] != "remove" || ops[2]["path"] != "/data/password-previous" {
		t.Errorf("want the previous key removed, got %v", ops[2])
	}
	if ops[3]["op"] != "add" || ops[3]["path"] != "/data/password-next" {
		t.Errorf("want the next key added first, so the remove works when it is missing, got %v", ops[3])
	}
	if ops[4]["op"] != "remove" || ops[4]["path"] != "/data/password-next" {
		t.Errorf("want the next key removed, got %v", ops[4])
	}
	if ops[5]["op"] != "replace" || ops[5]["path"] != annotationPathWant || ops[5]["value"] != RotationStepIdle {
		t.Errorf("want the step set back to idle, got %v", ops[5])
	}
}

func TestRotationResetCopiedPatch(t *testing.T) {
	t.Parallel()

	body, err := RotationResetCopiedPatch(newTestCache(nil))
	ops := decodePatch(t, body, err)
	expectGuard(t, ops, RotationStepCopied)
	if len(ops) != 4 {
		t.Fatalf("want 4 operations, got %d: %v", len(ops), ops)
	}
	if ops[2]["op"] != "remove" || ops[2]["path"] != "/data/password-previous" {
		t.Errorf("want the copied previous key removed, got %v", ops[2])
	}
	if ops[3]["op"] != "replace" || ops[3]["path"] != annotationPathWant || ops[3]["value"] != RotationStepIdle {
		t.Errorf("want the step set back to idle, got %v", ops[3])
	}
}

func TestRotationStepProbe(t *testing.T) {
	t.Parallel()

	body, err := RotationStepProbe(newTestCache(nil), RotationStepReplaced)
	ops := decodePatch(t, body, err)
	expectGuard(t, ops, RotationStepReplaced)
	if len(ops) != 2 {
		t.Fatalf("the probe must change nothing: want 2 test operations, got %d: %v", len(ops), ops)
	}
}

func TestRotationAdoptPatch(t *testing.T) {
	t.Parallel()

	body, err := RotationAdoptPatch(newTestCache(nil))
	ops := decodePatch(t, body, err)
	if len(ops) != 2 {
		t.Fatalf("want 2 operations, got %d: %v", len(ops), ops)
	}
	if ops[0]["op"] != "test" || ops[0]["path"] != labelPathWant || ops[0]["value"] != "app-one-cache" {
		t.Errorf("want a test on the cache label first, got %v", ops[0])
	}
	if ops[1]["op"] != "add" || ops[1]["path"] != annotationPathWant || ops[1]["value"] != RotationStepIdle {
		t.Errorf("want the one idle annotation added, so other annotations stay, got %v", ops[1])
	}

	body, err = RotationAdoptMapPatch(newTestCache(nil))
	ops = decodePatch(t, body, err)
	if len(ops) != 2 || ops[0]["op"] != "test" || ops[1]["op"] != "add" || ops[1]["path"] != "/metadata/annotations" {
		t.Fatalf("want a label test and the annotations map added, got %v", ops)
	}
	if got, ok := ops[1]["value"].(map[string]any); !ok || len(got) != 1 || got[RotationStepAnnotation] != RotationStepIdle {
		t.Errorf("want only the idle annotation in the new map, got %v", ops[1]["value"])
	}
}

func TestPasswordKeysFollowTheRotationCondition(t *testing.T) {
	t.Parallel()

	withCondition := func(status metav1.ConditionStatus, reason string) *cachev1alpha1.Cache {
		return newTestCache(func(c *cachev1alpha1.Cache) {
			meta.SetStatusCondition(&c.Status.Conditions, metav1.Condition{
				Type:   string(cachev1alpha1.ConditionPasswordRotated),
				Status: status,
				Reason: reason,
			})
		})
	}
	one := []string{AuthSecretPasswordKey}
	all := []string{AuthSecretPasswordKey, AuthSecretPreviousPasswordKey, AuthSecretNextPasswordKey}

	cases := []struct {
		name  string
		cache *cachev1alpha1.Cache
		want  []string
	}{
		{"no condition", newTestCache(nil), one},
		{"rotated", withCondition(metav1.ConditionTrue, "Rotated"), one},
		{"rotating, stage not confirmed", withCondition(metav1.ConditionFalse, "Rotating"), one},
		{"staged", withCondition(metav1.ConditionFalse, ReasonPasswordStaged), all},
		{"previous valid", withCondition(metav1.ConditionFalse, ReasonPreviousPasswordValid), []string{AuthSecretPasswordKey, AuthSecretPreviousPasswordKey}},
		{"legacy copied", withCondition(metav1.ConditionFalse, "PasswordCopied"), one},
		{"failed", withCondition(metav1.ConditionFalse, "RotationFailed"), one},
	}
	for _, tc := range cases {
		if got := PasswordKeys(tc.cache); !slices.Equal(got, tc.want) {
			t.Errorf("%s: want keys %v, got %v", tc.name, tc.want, got)
		}
	}
}

func TestEscapeJSONPointer(t *testing.T) {
	t.Parallel()

	if got := escapeJSONPointer("a/b~c"); got != "a~1b~0c" {
		t.Errorf("want a~1b~0c, got %q", got)
	}
}
