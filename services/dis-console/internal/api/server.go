// Package api serves the read-only JSON API backed by the PostgreSQL store the
// poller writes to. The poller calls MarkSynced after each successful sweep so
// /readyz reports ready only once data has been persisted and the DB pings.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/store"
)

// readyzPingTimeout bounds the DB ping done by /readyz.
const readyzPingTimeout = 3 * time.Second

// Store is the read surface the API needs from the persistence layer. The
// concrete *store.Store satisfies it; tests use an in-memory fake.
type Store interface {
	Summary(ctx context.Context) ([]store.KindCount, error)
	List(ctx context.Context, kind, namespace, ready string) ([]flux.Resource, error)
	Get(ctx context.Context, kind, namespace, name string) (*flux.Resource, []store.Event, error)
	LastSweep(ctx context.Context) (time.Time, error)
	Ping(ctx context.Context) error
}

// Server serves the API from the store.
type Server struct {
	store Store
	ready atomic.Bool
}

// NewServer returns a Server backed by st. /readyz reports not-ready until
// MarkSynced is first called.
func NewServer(st Store) *Server {
	return &Server{store: st}
}

// MarkSynced records that this process has persisted at least one sweep, which
// flips /readyz to ready. The "as of" timestamp the API reports comes from the
// database (LastSweep), not from here, so it survives restarts.
func (s *Server) MarkSynced() {
	s.ready.Store(true)
}

// updatedAt is the timestamp of the most recent persisted sweep, read from the
// database. On error it logs and returns the zero time rather than failing the
// data response (the timestamp is metadata, not the payload).
func (s *Server) updatedAt(r *http.Request) time.Time {
	t, err := s.store.LastSweep(r.Context())
	if err != nil {
		log.Printf("last sweep query failed: %v", err)
		return time.Time{}
	}
	return t
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

type detailResponse struct {
	flux.Resource
	History []store.Event `json:"history"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleReadyz reports ready only once the first sweep has been persisted and
// the DB still pings. Liveness (/healthz) stays 200 regardless, so a transient
// DB blip does not get the pod killed.
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

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	counts, err := s.store.Summary(r.Context())
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
	writeJSON(w, http.StatusOK, summaryResponse{
		UpdatedAt: s.updatedAt(r),
		Total:     total,
		Kinds:     kinds,
	})
}

func (s *Server) handleResources(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.writeFiltered(w, r, q.Get("kind"), q.Get("namespace"), q.Get("ready"))
}

func (s *Server) handleKustomizations(w http.ResponseWriter, r *http.Request) {
	s.writeFiltered(w, r, flux.KindKustomization, r.URL.Query().Get("namespace"), r.URL.Query().Get("ready"))
}

func (s *Server) handleHelmReleases(w http.ResponseWriter, r *http.Request) {
	s.writeFiltered(w, r, flux.KindHelmRelease, r.URL.Query().Get("namespace"), r.URL.Query().Get("ready"))
}

func (s *Server) writeFiltered(w http.ResponseWriter, r *http.Request, kind, namespace, ready string) {
	rows, err := s.store.List(r.Context(), kind, namespace, ready)
	if err != nil {
		s.fail(w, "list", err)
		return
	}
	writeJSON(w, http.StatusOK, listResponse{
		UpdatedAt: s.updatedAt(r),
		Count:     len(rows),
		Resources: rows,
	})
}

func (s *Server) handleResourceDetail(w http.ResponseWriter, r *http.Request) {
	res, history, err := s.store.Get(r.Context(), r.PathValue("kind"), r.PathValue("namespace"), r.PathValue("name"))
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody("resource not found"))
		return
	}
	if err != nil {
		s.fail(w, "get", err)
		return
	}
	writeJSON(w, http.StatusOK, detailResponse{Resource: *res, History: history})
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
