package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func do(t *testing.T, s *Server, path string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code
}

func TestHealthzAlwaysOK(t *testing.T) {
	s := New(fakePinger{}) // not ready, healthz still 200
	if code := do(t, s, "/healthz"); code != http.StatusOK {
		t.Fatalf("healthz: expected 200, got %d", code)
	}
}

func TestReadyzBeforeMarkReady(t *testing.T) {
	s := New(fakePinger{})
	if code := do(t, s, "/readyz"); code != http.StatusServiceUnavailable {
		t.Fatalf("readyz before ready: expected 503, got %d", code)
	}
}

func TestReadyzAfterMarkReady(t *testing.T) {
	s := New(fakePinger{})
	s.MarkReady()
	if code := do(t, s, "/readyz"); code != http.StatusOK {
		t.Fatalf("readyz after ready: expected 200, got %d", code)
	}
}

func TestReadyzPingFailure(t *testing.T) {
	s := New(fakePinger{err: errors.New("db down")})
	s.MarkReady()
	if code := do(t, s, "/readyz"); code != http.StatusServiceUnavailable {
		t.Fatalf("readyz with failing ping: expected 503, got %d", code)
	}
}
