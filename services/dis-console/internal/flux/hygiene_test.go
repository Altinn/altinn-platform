package flux

import (
	"encoding/json"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// withVolatile builds a Kustomization carrying the churn-only metadata fields
// plus the given resourceVersion, so tests can vary just those.
func withVolatile(resourceVersion string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata": map[string]any{
			"namespace":       "flux-system",
			"name":            "apps",
			"generation":      int64(3),
			"resourceVersion": resourceVersion,
			"managedFields": []any{
				map[string]any{"manager": "kustomize-controller", "operation": "Update"},
			},
		},
		"status": map[string]any{
			"observedGeneration": int64(3),
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "True", "reason": "ReconciliationSucceeded"},
			},
		},
	}}
}

func TestNormalizeStripsVolatileMetadata(t *testing.T) {
	r, err := normalize(withVolatile("12345"))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	var stored map[string]any
	if err := json.Unmarshal(r.Raw, &stored); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	md, ok := stored["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("missing metadata in raw: %v", stored)
	}
	if _, present := md["managedFields"]; present {
		t.Fatalf("managedFields should be stripped from raw: %v", md)
	}
	if _, present := md["resourceVersion"]; present {
		t.Fatalf("resourceVersion should be stripped from raw: %v", md)
	}
	// Non-volatile fields survive.
	if md["name"] != "apps" || md["generation"] == nil {
		t.Fatalf("expected name/generation preserved: %v", md)
	}
}

func TestContentHashIgnoresVolatileMetadata(t *testing.T) {
	// Two sweeps of the "same" object that differ only in the volatile fields
	// must hash identically, so the store treats them as unchanged.
	a, err := normalize(withVolatile("12345"))
	if err != nil {
		t.Fatalf("normalize a: %v", err)
	}
	b, err := normalize(withVolatile("99999"))
	if err != nil {
		t.Fatalf("normalize b: %v", err)
	}
	if a.ContentHash == "" {
		t.Fatalf("expected a content hash")
	}
	if a.ContentHash != b.ContentHash {
		t.Fatalf("content hash should ignore volatile metadata: %q != %q", a.ContentHash, b.ContentHash)
	}
}

func TestContentHashChangesWithStatus(t *testing.T) {
	a, err := normalize(withVolatile("1"))
	if err != nil {
		t.Fatalf("normalize a: %v", err)
	}

	changed := withVolatile("1")
	conds := changed.Object["status"].(map[string]any)["conditions"].([]any)
	conds[0].(map[string]any)["status"] = "False"
	b, err := normalize(changed)
	if err != nil {
		t.Fatalf("normalize b: %v", err)
	}
	if a.ContentHash == b.ContentHash {
		t.Fatalf("content hash should change when status changes: %q", a.ContentHash)
	}
}

func TestNormalizeCapsOversizedRaw(t *testing.T) {
	big := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata":   map[string]any{"namespace": "flux-system", "name": "huge"},
		"spec":       map[string]any{"blob": strings.Repeat("x", MaxRawBytes+1024)},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False", "reason": "Boom"},
			},
		},
	}}

	r, err := normalize(big)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	// Status is still projected from the full object before truncation.
	if r.Ready != ReadyFalse || r.Reason != "Boom" {
		t.Fatalf("expected status projected from full object, got ready=%q reason=%q", r.Ready, r.Reason)
	}
	if len(r.Raw) > MaxRawBytes {
		t.Fatalf("stored raw should be capped, got %d bytes", len(r.Raw))
	}

	var stub map[string]any
	if err := json.Unmarshal(r.Raw, &stub); err != nil {
		t.Fatalf("unmarshal stub: %v", err)
	}
	if stub["_disConsoleTruncated"] != true {
		t.Fatalf("expected truncation marker, got %v", stub)
	}
	if md, ok := stub["metadata"].(map[string]any); !ok || md["name"] != "huge" {
		t.Fatalf("expected identity preserved in stub, got %v", stub)
	}
	if r.ContentHash == "" {
		t.Fatalf("expected content hash on truncated row")
	}
}
