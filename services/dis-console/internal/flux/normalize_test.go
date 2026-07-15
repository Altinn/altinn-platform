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

func TestNormalizeStaleReadyDowngradedToUnknown(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata":   map[string]any{"namespace": "apps", "name": "stale", "generation": int64(5)},
		"status": map[string]any{
			"observedGeneration": int64(4),
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "True", "reason": "ReconciliationSucceeded"},
			},
		},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.Ready != ReadyUnknown {
		t.Fatalf("expected stale Ready downgraded to Unknown, got %q", r.Ready)
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

func TestNormalizeVaultResourceIDAndReady(t *testing.T) {
	armID := "/subscriptions/s1/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/kv-app"
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "vault.dis.altinn.cloud/v1alpha1",
		"kind":       "Vault",
		"metadata":   map[string]any{"namespace": "team-a", "name": "kv-app"},
		"status": map[string]any{
			"resourceId": armID,
			"conditions": []any{
				map[string]any{
					"type":               "Ready",
					"status":             "True",
					"reason":             "Provisioned",
					"lastTransitionTime": "2026-06-02T10:00:00Z",
				},
			},
		},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.Ready != ReadyTrue || r.Reason != "Provisioned" {
		t.Fatalf("unexpected ready condition: %+v", r)
	}
	if r.AzureResourceID != armID {
		t.Fatalf("azureResourceId: got %q, want %q", r.AzureResourceID, armID)
	}
	if r.Parent != nil {
		t.Fatalf("expected no parent for a Vault, got %+v", r.Parent)
	}
	if r.LastTransition == nil {
		t.Fatalf("expected lastTransition parsed")
	}
}

func TestNormalizeDatabaseParentFromServerRef(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "storage.dis.altinn.cloud/v1alpha1",
		"kind":       "Database",
		"metadata":   map[string]any{"namespace": "team-a", "name": "appdb"},
		"spec": map[string]any{
			"name":   "appdb",
			"server": map[string]any{"name": "pg-main"},
		},
		"status": map[string]any{
			"observedGeneration": int64(1),
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False", "reason": "Provisioning"},
			},
		},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.Ready != ReadyFalse || r.Reason != "Provisioning" {
		t.Fatalf("unexpected ready condition: %+v", r)
	}
	if r.Parent == nil || r.Parent.Kind != KindDatabaseServer || r.Parent.Name != "pg-main" {
		t.Fatalf("unexpected parent: %+v", r.Parent)
	}
	if r.AzureResourceID != "" {
		t.Fatalf("expected no azureResourceId for a Database, got %q", r.AzureResourceID)
	}
}

func TestNormalizeApiVersionParentFromOwnerReference(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apim.dis.altinn.cloud/v1alpha1",
		"kind":       "ApiVersion",
		"metadata": map[string]any{
			"namespace": "team-a",
			"name":      "orders-v1",
			"ownerReferences": []any{
				map[string]any{
					"apiVersion": "apim.dis.altinn.cloud/v1alpha1",
					"kind":       "Api",
					"name":       "orders",
					"uid":        "u1",
					"controller": true,
				},
			},
		},
		"status": map[string]any{"provisioningState": "Succeeded"},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.Parent == nil || r.Parent.Kind != KindApi || r.Parent.Name != "orders" {
		t.Fatalf("unexpected parent: %+v", r.Parent)
	}
	if r.Ready != ReadyTrue || r.Reason != "Succeeded" {
		t.Fatalf("expected Succeeded mapped to ready True, got %+v", r)
	}
}

func TestNormalizeApimProvisioningState(t *testing.T) {
	cases := []struct {
		name      string
		kind      string
		status    map[string]any
		wantReady string
		wantARM   string
	}{
		{
			name:      "api succeeded with version set id",
			kind:      "Api",
			status:    map[string]any{"provisioningState": "Succeeded", "apiVersionSetID": "/subscriptions/s1/apiVersionSets/orders"},
			wantReady: ReadyTrue,
			wantARM:   "/subscriptions/s1/apiVersionSets/orders",
		},
		{
			name:      "backend failed with backend id",
			kind:      "Backend",
			status:    map[string]any{"provisioningState": "Failed", "backendID": "/subscriptions/s1/backends/orders-be"},
			wantReady: ReadyFalse,
			wantARM:   "/subscriptions/s1/backends/orders-be",
		},
		{
			name:      "transitional state stays unknown",
			kind:      "Api",
			status:    map[string]any{"provisioningState": "Updating"},
			wantReady: ReadyUnknown,
			wantARM:   "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "apim.dis.altinn.cloud/v1alpha1",
				"kind":       tc.kind,
				"metadata":   map[string]any{"namespace": "team-a", "name": "orders"},
				"status":     tc.status,
			}}
			r, err := normalize(u)
			if err != nil {
				t.Fatalf("normalize: %v", err)
			}
			if r.Ready != tc.wantReady {
				t.Fatalf("ready: got %q, want %q", r.Ready, tc.wantReady)
			}
			if state, _ := tc.status["provisioningState"].(string); r.Reason != state {
				t.Fatalf("reason: got %q, want the provisioning state %q", r.Reason, state)
			}
			if r.AzureResourceID != tc.wantARM {
				t.Fatalf("azureResourceId: got %q, want %q", r.AzureResourceID, tc.wantARM)
			}
		})
	}
}

// HelmReleases applied by a Kustomization carry the kustomize-controller
// ownership labels; the projected AppliedBy is what lets the list endpoint
// group child HelmReleases under their parent app (raw is detail-only).
func TestNormalizeHelmReleaseAppliedByFromLabels(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata": map[string]any{
			"namespace": "grafana",
			"name":      "grafana-operator",
			"labels": map[string]any{
				"kustomize.toolkit.fluxcd.io/name":      "grafana-operator-grafana-operator",
				"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
			},
		},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.AppliedBy == nil {
		t.Fatalf("expected AppliedBy populated from labels")
	}
	if r.AppliedBy.Name != "grafana-operator-grafana-operator" || r.AppliedBy.Namespace != "flux-system" {
		t.Fatalf("unexpected AppliedBy: %+v", r.AppliedBy)
	}
}

// A root Kustomization (or an Arc-managed object) carries no kustomize-controller
// labels; AppliedBy must stay nil so the JSON omits the field.
func TestNormalizeAppliedByEmptyWithoutLabels(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata":   map[string]any{"namespace": "flux-system", "name": "root"},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.AppliedBy != nil {
		t.Fatalf("expected nil AppliedBy without labels, got %+v", r.AppliedBy)
	}
}

// The base-layer projections: a Kustomization's sourceRef (namespace
// defaulted to its own) and its applied-object inventory, kept in Flux's
// compact entry shape.
func TestNormalizeKustomizationSourceRefAndInventory(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata":   map[string]any{"namespace": "product-team-a", "name": "team-a-ab12-team-a"},
		"spec": map[string]any{
			"sourceRef": map[string]any{"kind": "OCIRepository", "name": "team-a-ab12"},
		},
		"status": map[string]any{
			"inventory": map[string]any{
				"entries": []any{
					map[string]any{"id": "product-team-a_appdb_storage.dis.altinn.cloud_Database", "v": "v1alpha1"},
					map[string]any{"id": "product-team-a_app_apps_Deployment", "v": "v1"},
				},
			},
		},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.SourceRef == nil || r.SourceRef.Kind != KindOCIRepository || r.SourceRef.Name != "team-a-ab12" {
		t.Fatalf("unexpected sourceRef: %+v", r.SourceRef)
	}
	if r.SourceRef.Namespace != "product-team-a" {
		t.Fatalf("sourceRef namespace should default to the Kustomization's own, got %q", r.SourceRef.Namespace)
	}
	if len(r.Inventory) != 2 ||
		r.Inventory[0].ID != "product-team-a_appdb_storage.dis.altinn.cloud_Database" ||
		r.Inventory[0].Version != "v1alpha1" ||
		r.Inventory[1].ID != "product-team-a_app_apps_Deployment" {
		t.Fatalf("unexpected inventory: %+v", r.Inventory)
	}
}

// An explicit cross-namespace sourceRef is kept as-is; a Kustomization without
// an inventory (never reconciled) projects nil so the JSON omits the field.
func TestNormalizeKustomizationSourceRefExplicitNamespace(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata":   map[string]any{"namespace": "platform-system", "name": "example-operator"},
		"spec": map[string]any{
			"sourceRef": map[string]any{"kind": "OCIRepository", "name": "example-operator", "namespace": "flux-system"},
		},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.SourceRef == nil || r.SourceRef.Namespace != "flux-system" {
		t.Fatalf("explicit sourceRef namespace should be kept, got %+v", r.SourceRef)
	}
	if r.Inventory != nil {
		t.Fatalf("expected nil inventory without status.inventory, got %+v", r.Inventory)
	}
}

// A source kind's url and the artifact's origin annotations (git revision and
// repository, stamped by `flux push artifact`) — the identity + provenance of
// a base-layer artifact. The mutable tag rides in revision; the digest there
// is the real version.
func TestNormalizeOCIRepositoryURLAndOrigin(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "OCIRepository",
		"metadata":   map[string]any{"namespace": "product-team-a", "name": "team-a-ab12"},
		"spec":       map[string]any{"url": "oci://registry.example.com/team-a/syncroot"},
		"status": map[string]any{
			"artifact": map[string]any{
				"revision": "at23@sha256:abc123",
				"metadata": map[string]any{
					"org.opencontainers.image.revision": "main/0c2a3b4",
					"org.opencontainers.image.source":   "https://git.example.com/team-a/syncroot",
				},
			},
		},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.SourceURL != "oci://registry.example.com/team-a/syncroot" {
		t.Fatalf("unexpected sourceUrl: %q", r.SourceURL)
	}
	if r.Revision != "at23@sha256:abc123" {
		t.Fatalf("unexpected revision: %q", r.Revision)
	}
	if r.OriginRevision != "main/0c2a3b4" || r.OriginSource != "https://git.example.com/team-a/syncroot" {
		t.Fatalf("unexpected origin: %q %q", r.OriginRevision, r.OriginSource)
	}
	if r.SourceRef != nil {
		t.Fatalf("sourceRef is a Kustomization projection, got %+v", r.SourceRef)
	}
}

// A HelmRepository projects its url too; an artifact without the origin
// annotations (not pushed by flux push) stays empty.
func TestNormalizeHelmRepositoryURLWithoutOrigin(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "HelmRepository",
		"metadata":   map[string]any{"namespace": "platform-system", "name": "example-charts"},
		"spec":       map[string]any{"url": "https://charts.example.com"},
		"status": map[string]any{
			"artifact": map[string]any{"revision": "sha256:abc"},
		},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.SourceURL != "https://charts.example.com" {
		t.Fatalf("unexpected sourceUrl: %q", r.SourceURL)
	}
	if r.OriginRevision != "" || r.OriginSource != "" {
		t.Fatalf("expected empty origin without annotations, got %q %q", r.OriginRevision, r.OriginSource)
	}
}

func TestNormalizeOCIRepositoryRevisionFromArtifact(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "OCIRepository",
		"metadata":   map[string]any{"namespace": "flux-system", "name": "podinfo"},
		"spec":       map[string]any{"suspend": true},
		"status": map[string]any{
			"artifact": map[string]any{"revision": "latest@sha256:abc123"},
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "True", "reason": "Succeeded"},
			},
		},
	}}

	r, err := normalize(u)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if r.Revision != "latest@sha256:abc123" {
		t.Fatalf("expected revision from artifact, got %q", r.Revision)
	}
	if r.Ready != ReadyTrue {
		t.Fatalf("expected ready True, got %q", r.Ready)
	}
	if !r.Suspended {
		t.Fatalf("expected suspended")
	}
}
