// Package health serves liveness and readiness probes for the agent. It is
// deliberately separate from the read API (internal/api): the agent is a writer
// with no API surface beyond these probes, so it depends only on this.
package health

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

// pingTimeout bounds the DB ping done by /readyz.
const pingTimeout = 3 * time.Second

// Pinger is the single dependency readiness needs: a database reachability check.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Server answers /healthz (liveness) and /readyz (readiness). Readiness reports
// not-ready until MarkReady is called and stays gated on the DB still pinging,
// so a transient DB blip fails readiness without failing liveness (which would
// get the pod killed).
type Server struct {
	pinger Pinger
	ready  atomic.Bool
}

// New returns a Server whose readiness pings p.
func New(p Pinger) *Server {
	return &Server{pinger: p}
}

// MarkReady flips readiness on; call it after the first successful sweep is
// persisted. It is safe to call repeatedly.
func (s *Server) MarkReady() {
	s.ready.Store(true)
}

// Handler returns the HTTP handler exposing the probe endpoints.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
	defer cancel()
	if err := s.pinger.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
