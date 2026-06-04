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

	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/store"
)

// fakeStore is an in-memory Store implementing the same filtering/summary
// semantics as the SQL store, so the HTTP handlers can be exercised without a
// database.
type fakeStore struct {
	resources []flux.Resource
	history   map[string][]store.Event
	pingErr   error
}

func (f *fakeStore) Ping(context.Context) error { return f.pingErr }

func (f *fakeStore) Summary(context.Context) ([]store.KindCount, error) {
	byKind := map[string]*store.KindCount{}
	order := []string{}
	for _, r := range f.resources {
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

func (f *fakeStore) List(_ context.Context, kind, namespace, ready string) ([]flux.Resource, error) {
	out := []flux.Resource{}
	for _, r := range f.resources {
		if kind != "" && !strings.EqualFold(r.Kind, kind) {
			continue
		}
		if namespace != "" && r.Namespace != namespace {
			continue
		}
		if ready != "" && !strings.EqualFold(r.Ready, ready) {
			continue
		}
		r.Raw = nil
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeStore) Get(_ context.Context, kind, namespace, name string) (*flux.Resource, []store.Event, error) {
	for i := range f.resources {
		r := f.resources[i]
		if strings.EqualFold(r.Kind, kind) && r.Namespace == namespace && r.Name == name {
			return &r, f.history[r.Name], nil
		}
	}
	return nil, nil, store.ErrNotFound
}

func testServer() *Server {
	s := NewServer(&fakeStore{
		resources: []flux.Resource{
			{Kind: "Kustomization", Namespace: "flux-system", Name: "apps", Ready: flux.ReadyTrue},
			{Kind: "Kustomization", Namespace: "apps", Name: "broken", Ready: flux.ReadyFalse, Reason: "ReconciliationFailed"},
			{Kind: "HelmRelease", Namespace: "apps", Name: "chart", Ready: flux.ReadyUnknown, Suspended: true},
		},
		history: map[string][]store.Event{
			"broken": {{Ready: flux.ReadyFalse, Reason: "ReconciliationFailed", ObservedAt: time.Now()}},
		},
	})
	s.MarkSynced(time.Now())
	return s
}

func TestReadyzBeforeSync(t *testing.T) {
	s := NewServer(&fakeStore{})
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 before first sync, got %d", rec.Code)
	}
}

func TestReadyzAfterSyncPingFails(t *testing.T) {
	s := NewServer(&fakeStore{pingErr: errors.New("db down")})
	s.MarkSynced(time.Now())
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when ping fails, got %d", rec.Code)
	}
}

func TestResourcesReadyFalseFilter(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/resources?ready=False", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
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
		t.Fatalf("expected total 3, got %d", resp.Total)
	}
	var ks *kindSummary
	for i := range resp.Kinds {
		if resp.Kinds[i].Kind == "Kustomization" {
			ks = &resp.Kinds[i]
		}
	}
	if ks == nil || ks.Ready != 1 || ks.NotReady != 1 {
		t.Fatalf("unexpected kustomization summary: %+v", ks)
	}
}

func TestResourceDetailWithHistory(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/resources/Kustomization/apps/broken", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var resp detailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Name != "broken" || len(resp.History) != 1 {
		t.Fatalf("unexpected detail: %+v", resp)
	}
}

func TestResourceDetailNotFound(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/resources/Kustomization/apps/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
