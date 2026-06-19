// Package api serves the read-only fleet JSON API backed by the central read
// model the server's sync loop fills. /readyz reports ready only once the first
// sync cycle has completed (MarkSynced) and the central database pings.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/central"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/store"
)

// readyzPingTimeout bounds the DB ping done by /readyz.
const readyzPingTimeout = 3 * time.Second

// Store is the read surface the fleet API needs from the central read model.
// The concrete *central.Store satisfies it; tests use an in-memory fake.
type Store interface {
	Clusters(ctx context.Context, staleAfter time.Duration) ([]central.Cluster, error)
	Summary(ctx context.Context, cluster string) ([]store.KindCount, error)
	List(ctx context.Context, cluster, kind, namespace, ready string) ([]central.Resource, error)
	Get(ctx context.Context, cluster, kind, namespace, name string) (*central.Resource, error)
	History(ctx context.Context, cluster, kind, namespace, name string) ([]store.Event, error)
	Ping(ctx context.Context) error
}

// Server serves the fleet API from the central store.
type Server struct {
	store      Store
	staleAfter time.Duration
	ready      atomic.Bool
}

// NewServer returns a Server backed by st. staleAfter is the threshold for
// flagging a cluster stale on /api/clusters. /readyz reports not-ready until
// MarkSynced is first called.
func NewServer(st Store, staleAfter time.Duration) *Server {
	return &Server{store: st, staleAfter: staleAfter}
}

// MarkSynced records that at least one sync cycle has completed, which flips
// /readyz to ready.
func (s *Server) MarkSynced() { s.ready.Store(true) }

// Routes returns the HTTP handler with all endpoints registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /api/clusters", s.handleClusters)
	mux.HandleFunc("GET /api/summary", s.handleSummary)
	mux.HandleFunc("GET /api/resources", s.handleResources)
	mux.HandleFunc("GET /api/resources/{cluster}/{kind}/{namespace}/{name}", s.handleResourceDetail)
	mux.HandleFunc("GET /api/kustomizations", s.handleKustomizations)
	mux.HandleFunc("GET /api/helmreleases", s.handleHelmReleases)
	return mux
}

type clustersResponse struct {
	Count    int               `json:"count"`
	Clusters []central.Cluster `json:"clusters"`
}

type listResponse struct {
	Count     int                `json:"count"`
	Resources []central.Resource `json:"resources"`
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
	Cluster string        `json:"cluster,omitempty"`
	Total   int           `json:"total"`
	Kinds   []kindSummary `json:"kinds"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleReadyz reports ready only once the first sync cycle has completed and
// the central DB still pings. Liveness (/healthz) stays 200 regardless, so a
// transient DB blip does not get the pod killed.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), readyzPingTimeout)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleClusters(w http.ResponseWriter, r *http.Request) {
	clusters, err := s.store.Clusters(r.Context(), s.staleAfter)
	if err != nil {
		s.fail(w, "clusters", err)
		return
	}
	writeJSON(w, http.StatusOK, clustersResponse{Count: len(clusters), Clusters: clusters})
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	cluster := r.URL.Query().Get("cluster")
	counts, err := s.store.Summary(r.Context(), cluster)
	if err != nil {
		s.fail(w, "summary", err)
		return
	}
	kinds := make([]kindSummary, 0, len(counts))
	total := 0
	for _, c := range counts {
		kinds = append(kinds, kindSummary{
			Kind:      c.Kind,
			Total:     c.Total,
			Ready:     c.Ready,
			NotReady:  c.NotReady,
			Unknown:   c.Unknown,
			Suspended: c.Suspended,
		})
		total += c.Total
	}
	writeJSON(w, http.StatusOK, summaryResponse{Cluster: cluster, Total: total, Kinds: kinds})
}

func (s *Server) handleResources(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.writeFiltered(w, r, q.Get("cluster"), q.Get("kind"), q.Get("namespace"), q.Get("ready"))
}

func (s *Server) handleKustomizations(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.writeFiltered(w, r, q.Get("cluster"), flux.KindKustomization, q.Get("namespace"), q.Get("ready"))
}

func (s *Server) handleHelmReleases(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.writeFiltered(w, r, q.Get("cluster"), flux.KindHelmRelease, q.Get("namespace"), q.Get("ready"))
}

func (s *Server) writeFiltered(w http.ResponseWriter, r *http.Request, cluster, kind, namespace, ready string) {
	rows, err := s.store.List(r.Context(), cluster, kind, namespace, ready)
	if err != nil {
		s.fail(w, "list", err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse{Count: len(rows), Resources: rows})
}

// resourceDetail is the detail-endpoint payload: the resource (flattened) plus
// its recent status history, newest first.
type resourceDetail struct {
	*central.Resource
	History []store.Event `json:"history"`
}

func (s *Server) handleResourceDetail(w http.ResponseWriter, r *http.Request) {
	cluster := r.PathValue("cluster")
	res, err := s.store.Get(r.Context(),
		cluster, r.PathValue("kind"), r.PathValue("namespace"), r.PathValue("name"))
	if errors.Is(err, central.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody("resource not found"))
		return
	}
	if err != nil {
		s.fail(w, "get", err)
		return
	}
	history, err := s.store.History(r.Context(), cluster, res.Kind, res.Namespace, res.Name)
	if err != nil {
		s.fail(w, "history", err)
		return
	}
	writeJSON(w, http.StatusOK, resourceDetail{Resource: res, History: history})
}

// fail logs the underlying error and returns a generic 500 to the client.
func (s *Server) fail(w http.ResponseWriter, op string, err error) {
	log.Printf("%s query failed: %v", op, err)
	writeJSON(w, http.StatusInternalServerError, errorBody(op+" query failed"))
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
