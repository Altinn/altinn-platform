package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/central"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/store"
)

// fakeStore is an in-memory Store implementing the same cluster/kind/namespace/
// ready filtering as the central SQL store, so the handlers run without a DB.
type fakeStore struct {
	resources []central.Resource
	clusters  []central.Cluster
	history   map[string][]store.Event
	pingErr   error
}

func (f *fakeStore) Ping(context.Context) error { return f.pingErr }

func (f *fakeStore) Clusters(context.Context, time.Duration) ([]central.Cluster, error) {
	return f.clusters, nil
}

func (f *fakeStore) Summary(_ context.Context, cluster string) ([]store.KindCount, error) {
	byKind := map[string]*store.KindCount{}
	order := []string{}
	for _, r := range f.resources {
		if cluster != "" && r.Cluster != cluster {
			continue
		}
		c, ok := byKind[r.Kind]
		if !ok {
			c = &store.KindCount{Kind: r.Kind}
			byKind[r.Kind] = c
			order = append(order, r.Kind)
		}
		c.Total++
		switch r.Ready {
		case flux.ReadyTrue:
			c.Ready++
		case flux.ReadyFalse:
			c.NotReady++
		default:
			c.Unknown++
		}
		if r.Suspended {
			c.Suspended++
		}
	}
	out := make([]store.KindCount, 0, len(order))
	for _, k := range order {
		out = append(out, *byKind[k])
	}
	return out, nil
}

func (f *fakeStore) List(_ context.Context, cluster, kind, namespace, ready string) ([]central.Resource, error) {
	out := []central.Resource{}
	for _, r := range f.resources {
		if cluster != "" && r.Cluster != cluster {
			continue
		}
		if kind != "" && !strings.EqualFold(r.Kind, kind) {
			continue
		}
		if namespace != "" && r.Namespace != namespace {
			continue
		}
		if ready != "" && !strings.EqualFold(r.Ready, ready) {
			continue
		}
		// The SQL list omits the detail-only payloads.
		r.Raw = nil
		r.Inventory = nil
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeStore) Get(_ context.Context, cluster, kind, namespace, name string) (*central.Resource, error) {
	for i := range f.resources {
		r := f.resources[i]
		if r.Cluster == cluster && strings.EqualFold(r.Kind, kind) && r.Namespace == namespace && r.Name == name {
			return &r, nil
		}
	}
	return nil, central.ErrNotFound
}

func histKey(cluster, kind, namespace, name string) string {
	return cluster + "|" + strings.ToLower(kind) + "|" + namespace + "|" + name
}

func (f *fakeStore) History(_ context.Context, cluster, kind, namespace, name string) ([]store.Event, error) {
	if h, ok := f.history[histKey(cluster, kind, namespace, name)]; ok {
		return h, nil
	}
	return []store.Event{}, nil
}

func res(cluster, kind, namespace, name, ready, reason string, suspended bool) central.Resource {
	return central.Resource{
		Cluster: cluster,
		Resource: flux.Resource{
			Kind: kind, Namespace: namespace, Name: name, Ready: ready, Reason: reason, Suspended: suspended,
		},
	}
}

func testServer() *Server {
	s := NewServer(&fakeStore{
		resources: []central.Resource{
			res("ttd_at23", "Kustomization", "flux-system", "apps", flux.ReadyTrue, "", false),
			res("ttd_at23", "Kustomization", "apps", "broken", flux.ReadyFalse, "ReconciliationFailed", false),
			res("skd_at23", "HelmRelease", "apps", "chart", flux.ReadyUnknown, "", true),
		},
		clusters: []central.Cluster{
			{Cluster: "ttd_at23", Environment: "at23", ResourceCount: 2, Stale: false},
			{Cluster: "skd_at23", Environment: "at23", ResourceCount: 1, Stale: true},
		},
		history: map[string][]store.Event{
			histKey("ttd_at23", "Kustomization", "apps", "broken"): {
				{Ready: flux.ReadyFalse, Reason: "ReconciliationFailed", ObservedAt: time.Now()},
				{Ready: flux.ReadyTrue, ObservedAt: time.Now().Add(-time.Hour)},
			},
		},
	}, time.Minute)
	s.MarkSynced()
	return s
}

func TestReadyzBeforeSync(t *testing.T) {
	s := NewServer(&fakeStore{}, time.Minute)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 before first sync, got %d", rec.Code)
	}
}

func TestReadyzAfterSyncPingFails(t *testing.T) {
	s := NewServer(&fakeStore{pingErr: errors.New("db down")}, time.Minute)
	s.MarkSynced()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when ping fails, got %d", rec.Code)
	}
}

func TestClusters(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/clusters", nil))
	var resp clustersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected 2 clusters, got %d", resp.Count)
	}
	var stale int
	for _, c := range resp.Clusters {
		if c.Stale {
			stale++
		}
	}
	if stale != 1 {
		t.Fatalf("expected 1 stale cluster, got %d", stale)
	}
}

func TestResourcesClusterFilter(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/resources?cluster=skd_at23", nil))
	var resp listResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Count != 1 || resp.Resources[0].Cluster != "skd_at23" {
		t.Fatalf("unexpected cluster filter result: %+v", resp)
	}
}

func TestResourcesReadyFalseFilter(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/resources?ready=False", nil))
	var resp listResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Count != 1 || resp.Resources[0].Name != "broken" {
		t.Fatalf("unexpected filter result: %+v", resp)
	}
}

func TestKustomizationsAlias(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/kustomizations", nil))
	var resp listResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected 2 kustomizations, got %d", resp.Count)
	}
}

func TestSummaryCounts(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/summary", nil))
	var resp summaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 3 {
		t.Fatalf("expected total 3 across the fleet, got %d", resp.Total)
	}

	// Scoped to one cluster.
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/summary?cluster=ttd_at23", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Cluster != "ttd_at23" || resp.Total != 2 {
		t.Fatalf("unexpected per-cluster summary: %+v", resp)
	}
}

func TestResourceDetail(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/resources/ttd_at23/Kustomization/apps/broken", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var resp central.Resource
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Name != "broken" || resp.Cluster != "ttd_at23" {
		t.Fatalf("unexpected detail: %+v", resp)
	}
}

func TestResourceDetailNotFound(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/resources/ttd_at23/Kustomization/apps/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestResourceDetailHistory(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/resources/ttd_at23/Kustomization/apps/broken", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var resp resourceDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Resource == nil || resp.Name != "broken" {
		t.Fatalf("unexpected detail: %+v", resp)
	}
	if len(resp.History) != 2 || resp.History[0].Ready != flux.ReadyFalse {
		t.Fatalf("expected 2 history events newest-first, got %+v", resp.History)
	}
}
