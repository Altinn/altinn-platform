// Package api serves the read-only JSON API from the latest in-memory
// snapshot of Flux resources produced by the poller.
package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
)

const noSnapshotMsg = "no snapshot yet"

// Snapshot is an immutable point-in-time view of all Flux resources.
type Snapshot struct {
	Resources []flux.Resource
	UpdatedAt time.Time
}

// Server holds the latest snapshot and serves it over HTTP.
type Server struct {
	snap atomic.Pointer[Snapshot]
}

// NewServer returns a Server with no snapshot yet; /readyz reports not-ready
// until SetSnapshot is first called.
func NewServer() *Server {
	return &Server{}
}

// SetSnapshot atomically replaces the served snapshot. Called by the poller
// after each sweep. The input slice is sorted in place by kind/namespace/name.
func (s *Server) SetSnapshot(resources []flux.Resource, updatedAt time.Time) {
	sort.Slice(resources, func(i, j int) bool {
		a, b := resources[i], resources[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
	s.snap.Store(&Snapshot{Resources: resources, UpdatedAt: updatedAt})
}

// Routes returns the HTTP handler with all endpoints registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /api/summary", s.handleSummary)
	mux.HandleFunc("GET /api/resources", s.handleResources)
	mux.HandleFunc("GET /api/resources/{kind}/{namespace}/{name}", s.handleResourceDetail)
	mux.HandleFunc("GET /api/kustomizations", s.handleKustomizations)
	mux.HandleFunc("GET /api/helmreleases", s.handleHelmReleases)
	return mux
}

type listResponse struct {
	UpdatedAt time.Time       `json:"updatedAt"`
	Count     int             `json:"count"`
	Resources []flux.Resource `json:"resources"`
}

type kindSummary struct {
	Kind      string `json:"kind"`
	Total     int    `json:"total"`
	Ready     int    `json:"ready"`
	NotReady  int    `json:"notReady"`
	Unknown   int    `json:"unknown"`
	Suspended int    `json:"suspended"`
}

type summaryResponse struct {
	UpdatedAt time.Time     `json:"updatedAt"`
	Total     int           `json:"total"`
	Kinds     []kindSummary `json:"kinds"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if s.snap.Load() == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSummary(w http.ResponseWriter, _ *http.Request) {
	snap := s.snap.Load()
	if snap == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody(noSnapshotMsg))
		return
	}
	// Resources are sorted by kind, so same-kind rows are contiguous.
	kinds := make([]kindSummary, 0, len(flux.TargetKinds))
	for _, res := range snap.Resources {
		if len(kinds) == 0 || kinds[len(kinds)-1].Kind != res.Kind {
			kinds = append(kinds, kindSummary{Kind: res.Kind})
		}
		k := &kinds[len(kinds)-1]
		k.Total++
		switch res.Ready {
		case flux.ReadyTrue:
			k.Ready++
		case flux.ReadyFalse:
			k.NotReady++
		default:
			k.Unknown++
		}
		if res.Suspended {
			k.Suspended++
		}
	}
	writeJSON(w, http.StatusOK, summaryResponse{
		UpdatedAt: snap.UpdatedAt,
		Total:     len(snap.Resources),
		Kinds:     kinds,
	})
}

func (s *Server) handleResources(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.writeFiltered(w, q.Get("kind"), q.Get("namespace"), q.Get("ready"))
}

func (s *Server) handleKustomizations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.writeFiltered(w, flux.KindKustomization, q.Get("namespace"), q.Get("ready"))
}

func (s *Server) handleHelmReleases(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.writeFiltered(w, flux.KindHelmRelease, q.Get("namespace"), q.Get("ready"))
}

func (s *Server) writeFiltered(w http.ResponseWriter, kind, namespace, ready string) {
	snap := s.snap.Load()
	if snap == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody(noSnapshotMsg))
		return
	}
	out := make([]flux.Resource, 0, len(snap.Resources))
	for _, res := range snap.Resources {
		if kind != "" && !strings.EqualFold(res.Kind, kind) {
			continue
		}
		if namespace != "" && res.Namespace != namespace {
			continue
		}
		if ready != "" && !strings.EqualFold(res.Ready, ready) {
			continue
		}
		res.Raw = nil // omit the heavy raw payload from list responses
		out = append(out, res)
	}
	writeJSON(w, http.StatusOK, listResponse{
		UpdatedAt: snap.UpdatedAt,
		Count:     len(out),
		Resources: out,
	})
}

func (s *Server) handleResourceDetail(w http.ResponseWriter, r *http.Request) {
	snap := s.snap.Load()
	if snap == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody(noSnapshotMsg))
		return
	}
	kind := r.PathValue("kind")
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	for i := range snap.Resources {
		res := snap.Resources[i]
		if strings.EqualFold(res.Kind, kind) && res.Namespace == namespace && res.Name == name {
			writeJSON(w, http.StatusOK, res)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, errorBody("resource not found"))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func errorBody(msg string) map[string]string {
	return map[string]string{"error": msg}
}
