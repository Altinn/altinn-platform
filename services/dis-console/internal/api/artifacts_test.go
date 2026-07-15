package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/central"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
)

// The fixtures use example hosts/teams on purpose; only the path shapes
// (<owner>/syncroot, manifests/infra/, dis/kustomize/) are the real contract
// the classifier keys on.
func TestClassifyArtifact(t *testing.T) {
	cases := []struct {
		url, class, owner string
	}{
		{"oci://registry.example.com/team-a/syncroot", ClassProductSyncroot, "team-a"},
		{"oci://registry.example.com/org/team-a/syncroot", ClassProductSyncroot, "org/team-a"},
		{"oci://registry.example.com/team-a/syncroot-admin", ClassAdminSyncroot, "team-a"},
		{"oci://registry.example.com/manifests/infra/example-package", ClassInfra, "example-package"},
		{"oci://registry.example.com/dis/kustomize/example-operator", ClassOperator, "example-operator"},
		{"oci://registry.example.com/monitoring/dashboards", ClassOther, ""},
		{"oci://registry.example.com/standalone", ClassOther, ""},
		{"", ClassOther, ""},
	}
	for _, tc := range cases {
		class, owner := classifyArtifact(tc.url)
		if class != tc.class || owner != tc.owner {
			t.Fatalf("classify(%q): got (%q, %q), want (%q, %q)", tc.url, class, owner, tc.class, tc.owner)
		}
	}
}

// artifactsTestServer mirrors a small fleet: a product syncroot on two
// clusters (one with its deploying Kustomization), an operator artifact, and a
// git-sourced Kustomization that must not attach to any artifact.
func artifactsTestServer() *Server {
	syncrootRepo := central.Resource{Cluster: "team-a_at23", Resource: flux.Resource{
		Kind: "OCIRepository", Namespace: "product-team-a", Name: "team-a-ab12",
		Ready: flux.ReadyTrue, Revision: "at23@sha256:abc",
		SourceURL:      "oci://registry.example.com/team-a/syncroot",
		OriginRevision: "main/0c2a3b4", OriginSource: "https://git.example.com/team-a/syncroot",
	}}
	syncrootKust := central.Resource{Cluster: "team-a_at23", Resource: flux.Resource{
		Kind: "Kustomization", Namespace: "product-team-a", Name: "team-a-ab12-team-a",
		Ready: flux.ReadyFalse, Reason: "HealthCheckFailed", Revision: "at23@sha256:old",
		SourceRef: &flux.SourceRef{Kind: "OCIRepository", Name: "team-a-ab12", Namespace: "product-team-a"},
	}}
	operatorRepo := central.Resource{Cluster: "team-a_at23", Resource: flux.Resource{
		Kind: "OCIRepository", Namespace: "platform-system", Name: "example-operator",
		Ready: flux.ReadyTrue, Revision: "v1.2.3@sha256:def",
		SourceURL: "oci://registry.example.com/dis/kustomize/example-operator",
	}}
	gitKust := central.Resource{Cluster: "team-a_at23", Resource: flux.Resource{
		Kind: "Kustomization", Namespace: "flux-system", Name: "from-git",
		Ready:     flux.ReadyTrue,
		SourceRef: &flux.SourceRef{Kind: "GitRepository", Name: "repo", Namespace: "flux-system"},
	}}
	otherClusterRepo := central.Resource{Cluster: "team-b_at23", Resource: flux.Resource{
		Kind: "OCIRepository", Namespace: "product-team-b", Name: "team-b-cd34",
		Ready: flux.ReadyTrue, Revision: "at23@sha256:fed",
		SourceURL: "oci://registry.example.com/team-b/syncroot",
	}}
	s := NewServer(&fakeStore{
		resources: []central.Resource{syncrootRepo, syncrootKust, operatorRepo, gitKust, otherClusterRepo},
	}, time.Minute)
	s.MarkSynced()
	return s
}

func getArtifacts(t *testing.T, s *Server, target string) artifactsResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var resp artifactsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

func TestArtifacts(t *testing.T) {
	s := artifactsTestServer()
	resp := getArtifacts(t, s, "/api/artifacts")
	if resp.Count != 3 {
		t.Fatalf("expected 3 artifacts, got %d: %+v", resp.Count, resp.Artifacts)
	}

	byName := map[string]artifactView{}
	for _, a := range resp.Artifacts {
		byName[a.Name] = a
	}

	syncroot := byName["team-a-ab12"]
	if syncroot.Class != ClassProductSyncroot || syncroot.Owner != "team-a" {
		t.Fatalf("unexpected syncroot classification: %+v", syncroot)
	}
	if syncroot.Revision != "at23@sha256:abc" || syncroot.OriginRevision != "main/0c2a3b4" {
		t.Fatalf("unexpected syncroot versions: %+v", syncroot)
	}
	if len(syncroot.Kustomizations) != 1 {
		t.Fatalf("expected the deploying kustomization attached, got %+v", syncroot.Kustomizations)
	}
	k := syncroot.Kustomizations[0]
	if k.Name != "team-a-ab12-team-a" || k.Revision != "at23@sha256:old" || k.Ready != flux.ReadyFalse {
		t.Fatalf("unexpected attached kustomization: %+v", k)
	}

	operator := byName["example-operator"]
	if operator.Class != ClassOperator || operator.Owner != "example-operator" {
		t.Fatalf("unexpected operator classification: %+v", operator)
	}
	if operator.Kustomizations == nil || len(operator.Kustomizations) != 0 {
		t.Fatalf("expected empty (non-null) kustomizations, got %+v", operator.Kustomizations)
	}
}

func TestArtifactsFilters(t *testing.T) {
	s := artifactsTestServer()

	resp := getArtifacts(t, s, "/api/artifacts?cluster=team-b_at23")
	if resp.Count != 1 || resp.Artifacts[0].Cluster != "team-b_at23" {
		t.Fatalf("unexpected cluster filter result: %+v", resp)
	}

	resp = getArtifacts(t, s, "/api/artifacts?class=product-syncroot")
	if resp.Count != 2 {
		t.Fatalf("expected 2 product syncroots, got %+v", resp)
	}
	for _, a := range resp.Artifacts {
		if a.Class != ClassProductSyncroot {
			t.Fatalf("class filter leaked %+v", a)
		}
	}
}

// inventoryTestServer holds one Kustomization whose inventory spans a swept
// DIS kind (enriched), an unswept Deployment, and a cluster-scoped object.
func inventoryTestServer() *Server {
	kust := central.Resource{Cluster: "team-a_at23", Resource: flux.Resource{
		Kind: "Kustomization", Namespace: "product-team-a", Name: "team-a-ab12-team-a",
		Ready: flux.ReadyTrue, Revision: "at23@sha256:abc",
		Inventory: []flux.InventoryEntry{
			{ID: "product-team-a_appdb_storage.dis.altinn.cloud_Database", Version: "v1alpha1"},
			{ID: "product-team-a_app_apps_Deployment", Version: "v1"},
			{ID: "_allow-all_kyverno.io_ClusterPolicy", Version: "v1"},
		},
	}}
	empty := central.Resource{Cluster: "team-a_at23", Resource: flux.Resource{
		Kind: "Kustomization", Namespace: "product-team-a", Name: "never-reconciled",
		Ready: flux.ReadyUnknown,
	}}
	db := central.Resource{Cluster: "team-a_at23", Resource: flux.Resource{
		Kind: "Database", Namespace: "product-team-a", Name: "appdb",
		Ready: flux.ReadyTrue, Parent: &flux.ParentRef{Kind: "DatabaseServer", Name: "pg-main"},
	}}
	s := NewServer(&fakeStore{resources: []central.Resource{kust, empty, db}}, time.Minute)
	s.MarkSynced()
	return s
}

func getInventory(t *testing.T, s *Server, target string) (int, inventoryResponse) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	var resp inventoryResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return rec.Code, resp
}

func TestKustomizationInventory(t *testing.T) {
	s := inventoryTestServer()
	code, resp := getInventory(t, s, "/api/kustomizations/team-a_at23/product-team-a/team-a-ab12-team-a/inventory")
	if code != http.StatusOK {
		t.Fatalf("status: %d", code)
	}
	if resp.Count != 3 || resp.Revision != "at23@sha256:abc" {
		t.Fatalf("unexpected inventory response: %+v", resp)
	}

	swept := resp.Entries[0]
	if swept.Kind != "Database" || swept.Namespace != "product-team-a" || swept.Name != "appdb" ||
		swept.Group != "storage.dis.altinn.cloud" || swept.Version != "v1alpha1" {
		t.Fatalf("unexpected expanded entry: %+v", swept)
	}
	if swept.Resource == nil || swept.Resource.Ready != flux.ReadyTrue || swept.Resource.Parent == nil {
		t.Fatalf("swept entry should carry the mirrored row: %+v", swept.Resource)
	}

	unswept := resp.Entries[1]
	if unswept.Kind != "Deployment" || unswept.Group != "apps" || unswept.Resource != nil {
		t.Fatalf("unswept entry should expand without a mirrored row: %+v", unswept)
	}

	clusterScoped := resp.Entries[2]
	if clusterScoped.Kind != "ClusterPolicy" || clusterScoped.Namespace != "" || clusterScoped.Name != "allow-all" {
		t.Fatalf("unexpected cluster-scoped entry: %+v", clusterScoped)
	}
}

func TestKustomizationInventoryEmpty(t *testing.T) {
	s := inventoryTestServer()
	code, resp := getInventory(t, s, "/api/kustomizations/team-a_at23/product-team-a/never-reconciled/inventory")
	if code != http.StatusOK {
		t.Fatalf("status: %d", code)
	}
	if resp.Count != 0 || resp.Entries == nil || len(resp.Entries) != 0 {
		t.Fatalf("expected empty (non-null) entries, got %+v", resp)
	}
}

func TestKustomizationInventoryNotFound(t *testing.T) {
	s := inventoryTestServer()
	code, _ := getInventory(t, s, "/api/kustomizations/team-a_at23/product-team-a/missing/inventory")
	if code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}
