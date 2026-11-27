package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-messaging-canary/internal/probe"
	"github.com/prometheus/client_golang/prometheus"
)

func get(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code, rec.Body.String()
}

func TestHandler(t *testing.T) {
	registry := prometheus.NewRegistry()
	probe.NewMetrics(registry)
	h := handler(registry)

	if code, _ := get(t, h, "/healthz"); code != http.StatusOK {
		t.Errorf("/healthz: got %d, want 200", code)
	}
	code, body := get(t, h, "/metrics")
	if code != http.StatusOK {
		t.Fatalf("/metrics: got %d, want 200", code)
	}
	// A scrape before the first round trip must show every failure series at 0.
	for _, series := range []string{`{stage="send"} 0`, `{stage="receive"} 0`, `{stage="complete"} 0`} {
		if !strings.Contains(body, series) {
			t.Errorf("/metrics: no line with %s in:\n%s", series, body)
		}
	}
	if strings.Contains(body, "go_goroutines") {
		t.Error("/metrics: has Go runtime metrics, want only the metrics of the canary")
	}
}

// settings returns the environment of a canary that listens on addr. The host
// name of the endpoint cannot resolve, so no test talks to Azure.
func settings(addr string) func(string) string {
	env := map[string]string{"ENDPOINT": "sb-team-a.invalid", "QUEUE": "team-a.orders", "CANARY_METRICS_ADDR": addr}
	return func(name string) string { return env[name] }
}

// workloadIdentity sets the variables that the workload identity webhook
// gives a pod. The canary reads them at the start. It asks for no token before
// it has a connection.
func workloadIdentity(t *testing.T) {
	t.Helper()
	t.Setenv("AZURE_CLIENT_ID", "00000000-0000-0000-0000-000000000001")
	t.Setenv("AZURE_TENANT_ID", "00000000-0000-0000-0000-000000000002")
	t.Setenv("AZURE_FEDERATED_TOKEN_FILE", filepath.Join(t.TempDir(), "token"))
	t.Setenv("AZURE_AUTHORITY_HOST", "https://login.invalid/")
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

func TestRunServesHealthAndStopsWhenTheContextEnds(t *testing.T) {
	workloadIdentity(t)
	listener := listen(t)
	addr := listener.Addr().String()
	_ = listener.Close() // run listens on this free port

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- run(ctx, settings(addr)) }()

	for start := time.Now(); ; time.Sleep(20 * time.Millisecond) {
		if resp, err := http.Get("http://" + addr + "/healthz"); err == nil {
			_ = resp.Body.Close()
			break
		}
		select {
		case err := <-stopped:
			t.Fatalf("run stopped at the start: %v", err)
		default:
		}
		if time.Since(start) > 5*time.Second {
			t.Fatal("/healthz did not answer in 5 s")
		}
	}
	cancel()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("run: got %v, want no error", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not stop in 10 s after the context ended")
	}
}

func TestRunFailsAtTheStart(t *testing.T) {
	inUse := listen(t).Addr().String()
	tests := []struct {
		name     string
		identity bool   // the pod has a workload identity
		addr     string // CANARY_METRICS_ADDR
		wantErr  string // the start of the error
	}{
		{name: "no workload identity", addr: "127.0.0.1:0", wantErr: "create the workload identity credential"},
		{name: "the address is in use", identity: true, addr: inUse, wantErr: "CANARY_METRICS_ADDR"},
		{name: "the address is wrong", identity: true, addr: "no-port", wantErr: "CANARY_METRICS_ADDR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AZURE_CLIENT_ID", "")
			if tt.identity {
				workloadIdentity(t)
			}
			if err := run(t.Context(), settings(tt.addr)); err == nil || !strings.HasPrefix(err.Error(), tt.wantErr) {
				t.Fatalf("run: got %v, want an error that starts with %q", err, tt.wantErr)
			}
		})
	}
}
