package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
)

func testServer() *Server {
	s := NewServer()
	s.SetSnapshot([]flux.Resource{
		{Kind: "Kustomization", Namespace: "flux-system", Name: "apps", Ready: flux.ReadyTrue},
		{Kind: "Kustomization", Namespace: "apps", Name: "broken", Ready: flux.ReadyFalse, Reason: "ReconciliationFailed"},
		{Kind: "HelmRelease", Namespace: "apps", Name: "chart", Ready: flux.ReadyUnknown, Suspended: true},
	}, time.Now())
	return s
}

func TestReadyzBeforeSnapshot(t *testing.T) {
	s := NewServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 before first sweep, got %d", rec.Code)
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

func TestResourceDetailNotFound(t *testing.T) {
	s := testServer()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/resources/Kustomization/apps/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
