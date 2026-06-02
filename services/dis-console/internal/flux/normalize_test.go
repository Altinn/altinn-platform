package flux

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNormalizeKustomization(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata": map[string]any{
			"namespace":  "flux-system",
			"name":       "apps",
			"generation": int64(3),
		},
		"spec": map[string]any{
			"suspend": true,
		},
		"status": map[string]any{
			"observedGeneration":  int64(3),
			"lastAppliedRevision": "main@sha1:abc123",
			"conditions": []any{
				map[string]any{
					"type":               "Ready",
					"status":             "False",
					"reason":             "ReconciliationFailed",
					"message":            "build failed",
					"lastTransitionTime": "2026-06-02T10:00:00Z",
				},
			},
		},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.Kind != "Kustomization" || r.Namespace != "flux-system" || r.Name != "apps" {
		t.Fatalf("unexpected identity: %+v", r)
	}
	if r.Ready != ReadyFalse || r.Reason != "ReconciliationFailed" || r.Message != "build failed" {
		t.Fatalf("unexpected ready condition: %+v", r)
	}
	if r.Revision != "main@sha1:abc123" {
		t.Fatalf("unexpected revision: %q", r.Revision)
	}
	if !r.Suspended {
		t.Fatalf("expected suspended")
	}
	if r.Generation != 3 || r.ObservedGeneration != 3 {
		t.Fatalf("unexpected generations: %d/%d", r.Generation, r.ObservedGeneration)
	}
	if r.LastTransition == nil {
		t.Fatalf("expected lastTransition parsed")
	}
	if len(r.Raw) == 0 {
		t.Fatalf("expected raw preserved")
	}
}

func TestNormalizeHelmReleaseRevisionFromHistory(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata":   map[string]any{"namespace": "apps", "name": "foo"},
		"status": map[string]any{
			"history": []any{
				map[string]any{"chartVersion": "1.2.3"},
			},
		},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.Revision != "1.2.3" {
		t.Fatalf("expected revision from history, got %q", r.Revision)
	}
	if r.Ready != ReadyUnknown {
		t.Fatalf("expected Unknown ready when no condition, got %q", r.Ready)
	}
}
