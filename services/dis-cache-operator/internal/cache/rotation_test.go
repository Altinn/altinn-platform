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

func TestRotationCopyPatch(t *testing.T) {
	t.Parallel()

	body, err := RotationCopyPatch(newTestCache(nil))
	ops := decodePatch(t, body, err)
	expectGuard(t, ops, RotationStepIdle)
	if len(ops) != 4 {
		t.Fatalf("want 4 operations, got %d: %v", len(ops), ops)
	}
	if ops[2]["op"] != "copy" || ops[2]["from"] != "/data/password" || ops[2]["path"] != "/data/password-previous" {
		t.Errorf("want a copy of the password to the previous key, got %v", ops[2])
	}
	if _, hasValue := ops[2]["value"]; hasValue {
		t.Errorf("the copy must carry no value: the operator never sees the password, got %v", ops[2])
	}
	if ops[3]["op"] != "replace" || ops[3]["path"] != annotationPathWant || ops[3]["value"] != RotationStepCopied {
		t.Errorf("want the step set to copied, got %v", ops[3])
	}
}

func TestRotationReplacePatch(t *testing.T) {
	t.Parallel()

	body, err := RotationReplacePatch(newTestCache(nil), "new-password-456")
	ops := decodePatch(t, body, err)
	expectGuard(t, ops, RotationStepCopied)
	if len(ops) != 4 {
		t.Fatalf("want 4 operations, got %d: %v", len(ops), ops)
	}
	want := base64.StdEncoding.EncodeToString([]byte("new-password-456"))
	if ops[2]["op"] != "replace" || ops[2]["path"] != "/data/password" || ops[2]["value"] != want {
		t.Errorf("want the password replaced with the base64 value, got %v", ops[2])
	}
	if ops[3]["value"] != RotationStepReplaced {
		t.Errorf("want the step set to replaced, got %v", ops[3])
	}
}

func TestRotationRemovePatch(t *testing.T) {
	t.Parallel()

	body, err := RotationRemovePatch(newTestCache(nil))
	ops := decodePatch(t, body, err)
	expectGuard(t, ops, RotationStepReplaced)
	if len(ops) != 4 {
		t.Fatalf("want 4 operations, got %d: %v", len(ops), ops)
	}
	if ops[2]["op"] != "remove" || ops[2]["path"] != "/data/password-previous" {
		t.Errorf("want the previous key removed, got %v", ops[2])
	}
	if ops[3]["value"] != RotationStepIdle {
		t.Errorf("want the step set back to idle, got %v", ops[3])
	}
}

func TestRotationMarkIdlePatch(t *testing.T) {
	t.Parallel()

	body, err := RotationMarkIdlePatch()
	if err != nil {
		t.Fatalf("build patch: %v", err)
	}
	var patch struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &patch); err != nil {
		t.Fatalf("decode patch %s: %v", body, err)
	}
	if patch.Metadata.Annotations[RotationStepAnnotation] != RotationStepIdle {
		t.Errorf("want the idle annotation only, got %s", body)
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
	both := []string{AuthSecretPasswordKey, AuthSecretPreviousPasswordKey}

	cases := []struct {
		name  string
		cache *cachev1alpha1.Cache
		want  []string
	}{
		{"no condition", newTestCache(nil), one},
		{"rotated", withCondition(metav1.ConditionTrue, "Rotated"), one},
		{"rotating", withCondition(metav1.ConditionFalse, "Rotating"), both},
		{"previous valid", withCondition(metav1.ConditionFalse, "PreviousPasswordValid"), both},
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
