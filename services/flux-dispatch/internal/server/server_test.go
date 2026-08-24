package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/dedup"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/dispatch"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/githubauth"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/metrics"
)

const testInstallationID = "7891011"

// recordedDispatch is one repository_dispatch call the fake GitHub received.
type recordedDispatch struct {
	Path          string
	EventType     string         `json:"event_type"`
	ClientPayload map[string]any `json:"client_payload"`
}

// fakeGitHub stands in for the GitHub API: it mints installation tokens and
// records repository_dispatch calls, with a settable response status.
type fakeGitHub struct {
	server *httptest.Server

	mu             sync.Mutex
	dispatches     []recordedDispatch
	dispatchStatus int
	tokenStatus    int
	tokenCalls     int
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{dispatchStatus: http.StatusNoContent, tokenStatus: http.StatusCreated}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /app/installations/{id}/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.tokenCalls++
		status := f.tokenStatus
		f.mu.Unlock()

		if status != http.StatusCreated {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":      "ghs_installation_token",
			"expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		})
	})
	mux.HandleFunc("POST /repos/{owner}/{repo}/dispatches", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var recorded recordedDispatch
		_ = json.Unmarshal(raw, &recorded)
		recorded.Path = r.URL.Path

		f.mu.Lock()
		f.dispatches = append(f.dispatches, recorded)
		status := f.dispatchStatus
		f.mu.Unlock()

		w.WriteHeader(status)
	})

	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGitHub) setDispatchStatus(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dispatchStatus = status
}

func (f *fakeGitHub) setTokenStatus(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenStatus = status
}

func (f *fakeGitHub) recorded() []recordedDispatch {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedDispatch(nil), f.dispatches...)
}

func (f *fakeGitHub) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.dispatches)
}

// harness is a fully wired service in front of a fake GitHub.
type harness struct {
	t       *testing.T
	github  *fakeGitHub
	metrics *metrics.Metrics
	handler http.Handler
	logs    *bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return buildHarness(t, false)
}

// newDryRunHarness is newHarness with DRY_RUN semantics: the handler is wired
// to a live fake GitHub so dry-run tests can assert it receives zero calls.
func newDryRunHarness(t *testing.T) *harness {
	t.Helper()
	return buildHarness(t, true)
}

func buildHarness(t *testing.T, dryRun bool) *harness {
	t.Helper()

	github := newFakeGitHub(t)
	m := metrics.New()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	app := githubauth.New("123456", testInstallationID, keyPEM, github.server.URL, github.server.Client())
	dispatcher := dispatch.New(github.server.URL, app, github.server.Client())
	tracker := dedup.New(24*time.Hour, 10000, m.DedupEntries)

	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	srv := New(Options{
		ListenAddr:           ":8080",
		MetricsAddr:          ":9090",
		DefaultDispatchEvent: "flux-deploy",
		DryRun:               dryRun,
		Tracker:              tracker,
		Dispatcher:           dispatcher,
		Metrics:              m,
		Logger:               logger,
	})

	return &harness{t: t, github: github, metrics: m, handler: srv.Handler(), logs: logs}
}

// post runs body through the handler.
func (h *harness) post(body string) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/flux-events", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}

// eventOptions describes a webhook body to build.
type eventOptions struct {
	reason        string
	revision      string
	origin        string
	product       string
	env           string
	dispatchRepo  string
	dispatchEvent string
	message       string
	padding       int
}

func buildEvent(o eventOptions) string {
	metadata := map[string]string{}
	if o.origin != "" {
		metadata["kustomize.toolkit.fluxcd.io/originRevision"] = o.origin
	}
	if o.revision != "" {
		metadata["kustomize.toolkit.fluxcd.io/revision"] = o.revision
	}
	if o.product != "" {
		metadata["product"] = o.product
	}
	if o.env != "" {
		metadata["env"] = o.env
	}
	if o.dispatchRepo != "" {
		metadata["dispatch_repo"] = o.dispatchRepo
	}
	if o.dispatchEvent != "" {
		metadata["dispatch_event"] = o.dispatchEvent
	}
	if o.padding > 0 {
		metadata["padding"] = strings.Repeat("p", o.padding)
	}

	body := map[string]any{
		"involvedObject": map[string]string{
			"kind":      "Kustomization",
			"name":      "dialogporten-apps",
			"namespace": "product-dialogporten",
		},
		"severity":  "info",
		"reason":    o.reason,
		"message":   o.message,
		"metadata":  metadata,
		"timestamp": "2026-03-05T12:00:00Z",
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// successEvent is the canonical happy-path body.
func successEvent() string {
	return buildEvent(eventOptions{
		reason:       "ReconciliationSucceeded",
		revision:     "at23@sha256:aabbccdd",
		origin:       "main/abc1234def5678",
		product:      "dialogporten",
		env:          "at23",
		dispatchRepo: "Altinn/dialogporten",
		message:      "Applied revision at23@sha256:aabbccdd",
	})
}

// TestCornerCases walks the RFC 0010 corner-case table, one request each.
func TestCornerCases(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		githubStatus  int
		wantStatus    int
		wantDispatch  int
		wantEventType string
	}{
		{
			name:          "happy path success event",
			body:          successEvent(),
			wantStatus:    http.StatusOK,
			wantDispatch:  1,
			wantEventType: "flux-deploy",
		},
		{
			name: "failure reason routed to failure event type",
			body: buildEvent(eventOptions{
				reason:        "ReconciliationFailed",
				revision:      "at23@sha256:aabbccdd",
				origin:        "main/abc1234def5678",
				product:       "dialogporten",
				env:           "at23",
				dispatchRepo:  "Altinn/dialogporten",
				dispatchEvent: "flux-deploy-failed",
				message:       "kustomize build failed",
			}),
			wantStatus:    http.StatusOK,
			wantDispatch:  1,
			wantEventType: "flux-deploy-failed",
		},
		{
			name: "health check failure is an accepted failure reason",
			body: buildEvent(eventOptions{
				reason:        "HealthCheckFailed",
				revision:      "at23@sha256:aabbccdd",
				product:       "dialogporten",
				env:           "at23",
				dispatchRepo:  "Altinn/dialogporten",
				dispatchEvent: "flux-deploy-failed",
			}),
			wantStatus:    http.StatusOK,
			wantDispatch:  1,
			wantEventType: "flux-deploy-failed",
		},
		{
			name: "error event through a success alert is not dispatched",
			body: buildEvent(eventOptions{
				reason:       "ReconciliationFailed",
				revision:     "at23@sha256:aabbccdd",
				product:      "dialogporten",
				env:          "at23",
				dispatchRepo: "Altinn/dialogporten",
			}),
			wantStatus:   http.StatusOK,
			wantDispatch: 0,
		},
		{
			name: "success event through a failure alert is not dispatched",
			body: buildEvent(eventOptions{
				reason:        "ReconciliationSucceeded",
				revision:      "at23@sha256:aabbccdd",
				product:       "dialogporten",
				env:           "at23",
				dispatchRepo:  "Altinn/dialogporten",
				dispatchEvent: "flux-deploy-failed",
			}),
			wantStatus:   http.StatusOK,
			wantDispatch: 0,
		},
		{
			name: "unknown reason acknowledged and ignored",
			body: buildEvent(eventOptions{
				reason:       "Progressing",
				revision:     "at23@sha256:aabbccdd",
				product:      "dialogporten",
				env:          "at23",
				dispatchRepo: "Altinn/dialogporten",
			}),
			wantStatus:   http.StatusOK,
			wantDispatch: 0,
		},
		{
			name: "missing dispatch_repo",
			body: buildEvent(eventOptions{
				reason:   "ReconciliationSucceeded",
				revision: "at23@sha256:aabbccdd",
				product:  "dialogporten",
				env:      "at23",
			}),
			wantStatus:   http.StatusOK,
			wantDispatch: 0,
		},
		{
			name: "non-Altinn dispatch_repo",
			body: buildEvent(eventOptions{
				reason:       "ReconciliationSucceeded",
				revision:     "at23@sha256:aabbccdd",
				product:      "dialogporten",
				env:          "at23",
				dispatchRepo: "Evil/repo",
			}),
			wantStatus:   http.StatusOK,
			wantDispatch: 0,
		},
		{
			name: "dot-only dispatch_repo cannot traverse the API path",
			body: buildEvent(eventOptions{
				reason:       "ReconciliationSucceeded",
				revision:     "at23@sha256:aabbccdd",
				product:      "dialogporten",
				env:          "at23",
				dispatchRepo: "Altinn/..",
			}),
			wantStatus:   http.StatusOK,
			wantDispatch: 0,
		},
		{
			name: "current-dir dispatch_repo cannot traverse the API path",
			body: buildEvent(eventOptions{
				reason:       "ReconciliationSucceeded",
				revision:     "at23@sha256:aabbccdd",
				product:      "dialogporten",
				env:          "at23",
				dispatchRepo: "Altinn/.",
			}),
			wantStatus:   http.StatusOK,
			wantDispatch: 0,
		},
		{
			name: "malformed dispatch_repo",
			body: buildEvent(eventOptions{
				reason:       "ReconciliationSucceeded",
				revision:     "at23@sha256:aabbccdd",
				product:      "dialogporten",
				env:          "at23",
				dispatchRepo: "Altinn/../x",
			}),
			wantStatus:   http.StatusOK,
			wantDispatch: 0,
		},
		{
			name: "body over 64 KB",
			body: buildEvent(eventOptions{
				reason:       "ReconciliationSucceeded",
				revision:     "at23@sha256:aabbccdd",
				product:      "dialogporten",
				env:          "at23",
				dispatchRepo: "Altinn/dialogporten",
				padding:      70 * 1024,
			}),
			wantStatus:   http.StatusRequestEntityTooLarge,
			wantDispatch: 0,
		},
		{
			name:         "malformed JSON body is acknowledged",
			body:         `{"involvedObject":`,
			wantStatus:   http.StatusOK,
			wantDispatch: 0,
		},
		{
			name:         "github 500 is retryable",
			body:         successEvent(),
			githubStatus: http.StatusInternalServerError,
			wantStatus:   http.StatusBadGateway,
			wantDispatch: 1, // attempted, not accepted
		},
		{
			name:         "github 404 is a config error",
			body:         successEvent(),
			githubStatus: http.StatusNotFound,
			wantStatus:   http.StatusOK,
			wantDispatch: 1, // attempted, not accepted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			if tt.githubStatus != 0 {
				h.github.setDispatchStatus(tt.githubStatus)
			}

			rec := h.post(tt.body)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body %q)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := h.github.count(); got != tt.wantDispatch {
				t.Errorf("dispatch calls = %d, want %d", got, tt.wantDispatch)
			}
			if tt.wantEventType != "" {
				recorded := h.github.recorded()
				if len(recorded) == 0 {
					t.Fatal("no dispatch recorded")
				}
				if recorded[0].EventType != tt.wantEventType {
					t.Errorf("event_type = %q, want %q", recorded[0].EventType, tt.wantEventType)
				}
			}
		})
	}
}

func TestHappyPathPayloadAndMetrics(t *testing.T) {
	h := newHarness(t)

	rec := h.post(successEvent())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	recorded := h.github.recorded()
	if len(recorded) != 1 {
		t.Fatalf("dispatch calls = %d, want 1", len(recorded))
	}
	if recorded[0].Path != "/repos/Altinn/dialogporten/dispatches" {
		t.Errorf("path = %q, want /repos/Altinn/dialogporten/dispatches", recorded[0].Path)
	}
	for field, want := range map[string]string{
		"product":            "dialogporten",
		"environment":        "at23",
		"commit_sha":         "abc1234def5678",
		"revision":           "at23@sha256:aabbccdd",
		"kustomization_name": "dialogporten-apps",
		"reason":             "ReconciliationSucceeded",
		"message":            "Applied revision at23@sha256:aabbccdd",
	} {
		if got := recorded[0].ClientPayload[field]; got != want {
			t.Errorf("client_payload[%q] = %v, want %q", field, got, want)
		}
	}

	if got := testutil.ToFloat64(h.metrics.EventsReceived.WithLabelValues("ReconciliationSucceeded")); got != 1 {
		t.Errorf("events_received_total = %v, want 1", got)
	}
	if got := testutil.ToFloat64(h.metrics.Dispatches.WithLabelValues(
		"Altinn/dialogporten", "flux-deploy", "ReconciliationSucceeded")); got != 1 {
		t.Errorf("dispatches_total = %v, want 1", got)
	}
	if got := testutil.ToFloat64(h.metrics.DedupEntries); got != 1 {
		t.Errorf("dedup_entries = %v, want 1", got)
	}
	if got := testutil.CollectAndCount(h.metrics.DispatchDuration); got != 1 {
		t.Errorf("dispatch_duration series = %d, want 1", got)
	}

	// Structured logs carry the fields operators filter on.
	record := findLogRecord(t, h.logs.String(), "dispatched")
	for field, want := range map[string]any{
		"product": "dialogporten",
		"env":     "at23",
		"repo":    "Altinn/dialogporten",
		"reason":  "ReconciliationSucceeded",
		"outcome": "dispatched",
	} {
		if got := record[field]; got != want {
			t.Errorf("log field %q = %v, want %v", field, got, want)
		}
	}
}

func TestDuplicateDigestIsSkipped(t *testing.T) {
	h := newHarness(t)

	if rec := h.post(successEvent()); rec.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec.Code)
	}
	if rec := h.post(successEvent()); rec.Code != http.StatusOK {
		t.Fatalf("second request status = %d, want 200", rec.Code)
	}

	if got := h.github.count(); got != 1 {
		t.Errorf("dispatch calls = %d, want 1 (second event is a duplicate)", got)
	}
	if got := testutil.ToFloat64(h.metrics.DedupHits.WithLabelValues("ReconciliationSucceeded")); got != 1 {
		t.Errorf("dedup_hits_total = %v, want 1", got)
	}
	if got := testutil.ToFloat64(h.metrics.EventsReceived.WithLabelValues("ReconciliationSucceeded")); got != 2 {
		t.Errorf("events_received_total = %v, want 2", got)
	}
}

func TestSameDigestFailThenSucceedBothDispatch(t *testing.T) {
	h := newHarness(t)

	failure := buildEvent(eventOptions{
		reason:        "ReconciliationFailed",
		revision:      "at23@sha256:aabbccdd",
		origin:        "main/abc1234def5678",
		product:       "dialogporten",
		env:           "at23",
		dispatchRepo:  "Altinn/dialogporten",
		dispatchEvent: "flux-deploy-failed",
		message:       "kustomize build failed",
	})

	if rec := h.post(failure); rec.Code != http.StatusOK {
		t.Fatalf("failure request status = %d, want 200", rec.Code)
	}
	if rec := h.post(successEvent()); rec.Code != http.StatusOK {
		t.Fatalf("success request status = %d, want 200", rec.Code)
	}

	recorded := h.github.recorded()
	if len(recorded) != 2 {
		t.Fatalf("dispatch calls = %d, want 2", len(recorded))
	}
	if recorded[0].EventType != "flux-deploy-failed" {
		t.Errorf("first event_type = %q, want flux-deploy-failed", recorded[0].EventType)
	}
	if recorded[1].EventType != "flux-deploy" {
		t.Errorf("second event_type = %q, want flux-deploy", recorded[1].EventType)
	}
	if got := testutil.ToFloat64(h.metrics.DedupHits.WithLabelValues("ReconciliationFailed")); got != 0 {
		t.Errorf("dedup_hits_total{ReconciliationFailed} = %v, want 0", got)
	}
}

func TestRepeatedFailureSameDigestIsDeduplicated(t *testing.T) {
	h := newHarness(t)

	failure := buildEvent(eventOptions{
		reason:        "ReconciliationFailed",
		revision:      "at23@sha256:aabbccdd",
		product:       "dialogporten",
		env:           "at23",
		dispatchRepo:  "Altinn/dialogporten",
		dispatchEvent: "flux-deploy-failed",
	})

	h.post(failure)
	h.post(failure)

	if got := h.github.count(); got != 1 {
		t.Errorf("dispatch calls = %d, want 1", got)
	}
}

func TestTwoReposFromOneKustomizationBothDispatch(t *testing.T) {
	h := newHarness(t)

	base := eventOptions{
		reason:       "ReconciliationSucceeded",
		revision:     "at23@sha256:aabbccdd",
		origin:       "main/abc1234def5678",
		product:      "dialogporten",
		env:          "at23",
		dispatchRepo: "Altinn/dialogporten",
	}
	h.post(buildEvent(base))

	base.dispatchRepo = "Altinn/dialogporten-dashboard"
	h.post(buildEvent(base))

	recorded := h.github.recorded()
	if len(recorded) != 2 {
		t.Fatalf("dispatch calls = %d, want 2", len(recorded))
	}
	if recorded[1].Path != "/repos/Altinn/dialogporten-dashboard/dispatches" {
		t.Errorf("second path = %q", recorded[1].Path)
	}
}

func TestConsecutiveDeploysWithDifferentDigests(t *testing.T) {
	h := newHarness(t)

	for _, digest := range []string{"sha256:1111", "sha256:2222", "sha256:3333"} {
		body := buildEvent(eventOptions{
			reason:       "ReconciliationSucceeded",
			revision:     "at23@" + digest,
			product:      "dialogporten",
			env:          "at23",
			dispatchRepo: "Altinn/dialogporten",
		})
		if rec := h.post(body); rec.Code != http.StatusOK {
			t.Fatalf("status for %s = %d, want 200", digest, rec.Code)
		}
	}

	if got := h.github.count(); got != 3 {
		t.Errorf("dispatch calls = %d, want 3", got)
	}
}

func TestGitHubErrorMetrics(t *testing.T) {
	tests := []struct {
		name         string
		githubStatus int
		wantStatus   int
		wantCode     string
	}{
		{"server error", http.StatusInternalServerError, http.StatusBadGateway, "500"},
		{"not found", http.StatusNotFound, http.StatusOK, "404"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.github.setDispatchStatus(tt.githubStatus)

			rec := h.post(successEvent())
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := testutil.ToFloat64(h.metrics.DispatchErrors.WithLabelValues(
				"Altinn/dialogporten", "flux-deploy", tt.wantCode)); got != 1 {
				t.Errorf("dispatch_errors_total{%s} = %v, want 1", tt.wantCode, got)
			}
			// A failed dispatch must not be remembered, so a retry can succeed.
			if got := testutil.ToFloat64(h.metrics.DedupEntries); got != 0 {
				t.Errorf("dedup_entries = %v, want 0 after a failed dispatch", got)
			}
		})
	}
}

func TestGitHubAuthFailureIsRetryable(t *testing.T) {
	h := newHarness(t)
	h.github.setTokenStatus(http.StatusUnauthorized)

	rec := h.post(successEvent())
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
	if got := h.github.count(); got != 0 {
		t.Errorf("dispatch calls = %d, want 0", got)
	}
	if got := testutil.ToFloat64(h.metrics.GitHubAuthErrors); got != 1 {
		t.Errorf("github_auth_errors_total = %v, want 1", got)
	}
	// A rotated App key fails every dispatch; an alert on dispatch error rate
	// must see it, so the auth path also increments dispatch_errors_total.
	if got := testutil.ToFloat64(h.metrics.DispatchErrors.WithLabelValues(
		"Altinn/dialogporten", "flux-deploy", "auth")); got != 1 {
		t.Errorf("dispatch_errors_total{error_code=\"auth\"} = %v, want 1", got)
	}
	// No API call was made, so the latency histogram must stay empty.
	if got := testutil.CollectAndCount(h.metrics.DispatchDuration); got != 0 {
		t.Errorf("dispatch_duration series = %d, want 0 on the auth path", got)
	}
}

func TestRetryAfterTransientFailureSucceeds(t *testing.T) {
	h := newHarness(t)
	h.github.setDispatchStatus(http.StatusInternalServerError)

	if rec := h.post(successEvent()); rec.Code != http.StatusBadGateway {
		t.Fatalf("first status = %d, want 502", rec.Code)
	}

	h.github.setDispatchStatus(http.StatusNoContent)
	if rec := h.post(successEvent()); rec.Code != http.StatusOK {
		t.Fatalf("retry status = %d, want 200", rec.Code)
	}

	if got := h.github.count(); got != 2 {
		t.Errorf("dispatch calls = %d, want 2 (the retry must not be deduplicated)", got)
	}
}

func TestLongMessageIsTruncated(t *testing.T) {
	h := newHarness(t)

	body := buildEvent(eventOptions{
		reason:        "ReconciliationFailed",
		revision:      "at23@sha256:aabbccdd",
		product:       "dialogporten",
		env:           "at23",
		dispatchRepo:  "Altinn/dialogporten",
		dispatchEvent: "flux-deploy-failed",
		message:       strings.Repeat("e", 8000),
	})
	if rec := h.post(body); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	recorded := h.github.recorded()
	if len(recorded) != 1 {
		t.Fatalf("dispatch calls = %d, want 1", len(recorded))
	}
	message, _ := recorded[0].ClientPayload["message"].(string)
	if len(message) != 1024 {
		t.Errorf("message length = %d, want 1024", len(message))
	}
}

func TestHealthEndpoints(t *testing.T) {
	h := newHarness(t)

	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, rec.Code)
		}
	}
}

func TestWebhookRejectsNonPost(t *testing.T) {
	h := newHarness(t)

	req := httptest.NewRequest(http.MethodGet, "/flux-events", nil)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /flux-events = %d, want 405", rec.Code)
	}
}

// TestMetricsHandlerExposesAllSevenCollectors drives the handler down every
// path that feeds a collector — a Prometheus vector with no observed labels is
// omitted from the exposition, so each one needs a real sample.
func TestMetricsHandlerExposesAllSevenCollectors(t *testing.T) {
	h := newHarness(t)

	// events_received, dispatches, dedup_entries, dispatch_duration.
	h.post(successEvent())
	// dedup_hits.
	h.post(successEvent())

	// dispatch_errors: a repo the App is not installed on.
	h.github.setDispatchStatus(http.StatusNotFound)
	h.post(buildEvent(eventOptions{
		reason:       "ReconciliationSucceeded",
		revision:     "at23@sha256:11111111",
		product:      "dialogporten",
		env:          "at23",
		dispatchRepo: "Altinn/dialogporten",
	}))

	// github_auth_errors.
	h.github.setTokenStatus(http.StatusUnauthorized)
	h.post(buildEvent(eventOptions{
		reason:       "ReconciliationSucceeded",
		revision:     "at23@sha256:22222222",
		product:      "dialogporten",
		env:          "at23",
		dispatchRepo: "Altinn/dialogporten",
	}))

	srv := New(Options{Metrics: h.metrics, Logger: slog.Default()})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	srv.MetricsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	for _, name := range []string{
		"flux_dispatch_events_received_total",
		"flux_dispatch_dispatches_total",
		"flux_dispatch_dispatch_errors_total",
		"flux_dispatch_dedup_hits_total",
		"flux_dispatch_dedup_entries",
		"flux_dispatch_github_auth_errors_total",
		"flux_dispatch_dispatch_duration_seconds",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("/metrics output is missing %s", name)
		}
	}
}

// TestServerHardening pins the timeouts RFC 0010 requires.
func TestServerHardening(t *testing.T) {
	srv := New(Options{
		ListenAddr:  ":8080",
		MetricsAddr: ":9090",
		Metrics:     metrics.New(),
		Logger:      slog.Default(),
	})

	webhook := srv.WebhookServer()
	if webhook.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", webhook.Addr)
	}
	if webhook.ReadTimeout != 10*time.Second {
		t.Errorf("ReadTimeout = %v, want 10s", webhook.ReadTimeout)
	}
	if webhook.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 5s", webhook.ReadHeaderTimeout)
	}
	if webhook.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v, want 30s", webhook.WriteTimeout)
	}
	if webhook.MaxHeaderBytes != 1<<16 {
		t.Errorf("MaxHeaderBytes = %d, want %d", webhook.MaxHeaderBytes, 1<<16)
	}

	metricsServer := srv.MetricsServer()
	if metricsServer.Addr != ":9090" {
		t.Errorf("metrics Addr = %q, want :9090", metricsServer.Addr)
	}
	if metricsServer.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("metrics ReadHeaderTimeout = %v, want 5s", metricsServer.ReadHeaderTimeout)
	}
}

// TestConcurrentEvents exercises the handler from several goroutines so
// `go test -race` covers the shared dedup tracker and metrics.
func TestConcurrentEvents(t *testing.T) {
	h := newHarness(t)

	var wg sync.WaitGroup
	for worker := range 4 {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := range 10 {
				body := buildEvent(eventOptions{
					reason:       "ReconciliationSucceeded",
					revision:     fmt.Sprintf("at23@sha256:%d%d", worker, i),
					product:      "dialogporten",
					env:          "at23",
					dispatchRepo: "Altinn/dialogporten",
				})
				h.post(body)
			}
		}(worker)
	}
	wg.Wait()

	if got := h.github.count(); got != 40 {
		t.Errorf("dispatch calls = %d, want 40", got)
	}
}

// TestContextIsPropagated makes sure a client disconnect aborts the outbound
// call rather than leaking it.
func TestContextIsPropagated(t *testing.T) {
	h := newHarness(t)

	body := successEvent()
	req := httptest.NewRequest(http.MethodPost, "/flux-events", strings.NewReader(body))

	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Errorf("status = 200, want a failure after the client went away")
	}
}

// TestDryRunHappyPathSkipsGitHub pins the dry-run contract: the outbound
// GitHub call is skipped, and flux_dispatch_dryrun_dispatches_total
// increments while flux_dispatch_dispatches_total must not move at all.
func TestDryRunHappyPathSkipsGitHub(t *testing.T) {
	h := newDryRunHarness(t)

	rec := h.post(successEvent())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if got := h.github.count(); got != 0 {
		t.Errorf("github dispatch calls = %d, want 0", got)
	}
	if got := testutil.ToFloat64(h.metrics.DryRunDispatches.WithLabelValues(
		"Altinn/dialogporten", "flux-deploy", "ReconciliationSucceeded")); got != 1 {
		t.Errorf("flux_dispatch_dryrun_dispatches_total = %v, want 1", got)
	}
	if got := testutil.ToFloat64(h.metrics.Dispatches.WithLabelValues(
		"Altinn/dialogporten", "flux-deploy", "ReconciliationSucceeded")); got != 0 {
		t.Errorf("flux_dispatch_dispatches_total = %v, want 0 (must not move in dry run)", got)
	}
	if got := testutil.ToFloat64(h.metrics.EventsReceived.WithLabelValues("ReconciliationSucceeded")); got != 1 {
		t.Errorf("events_received_total = %v, want 1", got)
	}
}

// TestDryRunDuplicateDigestIsSkipped proves dedup is fully observable in
// DRY_RUN mode, per the DECISION doc's explicit reason for the mode: the dedup
// key is recorded exactly as a successful dispatch would, so a repeat
// delivery of the same digest is a dedup hit with no second dry-run dispatch.
func TestDryRunDuplicateDigestIsSkipped(t *testing.T) {
	h := newDryRunHarness(t)

	if rec := h.post(successEvent()); rec.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec.Code)
	}
	if rec := h.post(successEvent()); rec.Code != http.StatusOK {
		t.Fatalf("second request status = %d, want 200", rec.Code)
	}

	if got := h.github.count(); got != 0 {
		t.Errorf("github dispatch calls = %d, want 0", got)
	}
	if got := testutil.ToFloat64(h.metrics.DedupHits.WithLabelValues("ReconciliationSucceeded")); got != 1 {
		t.Errorf("dedup_hits_total = %v, want 1", got)
	}
	if got := testutil.ToFloat64(h.metrics.DryRunDispatches.WithLabelValues(
		"Altinn/dialogporten", "flux-deploy", "ReconciliationSucceeded")); got != 1 {
		t.Errorf("flux_dispatch_dryrun_dispatches_total = %v, want 1 (second delivery is a duplicate)", got)
	}
}

// TestDryRunStillRejectsMissingRepo confirms product-team misconfiguration
// is still caught in DRY_RUN mode: only the outbound GitHub call is skipped.
func TestDryRunStillRejectsMissingRepo(t *testing.T) {
	h := newDryRunHarness(t)

	body := buildEvent(eventOptions{
		reason:   "ReconciliationSucceeded",
		revision: "at23@sha256:aabbccdd",
		product:  "dialogporten",
		env:      "at23",
	})
	rec := h.post(body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := h.github.count(); got != 0 {
		t.Errorf("github dispatch calls = %d, want 0", got)
	}
	if got := testutil.CollectAndCount(h.metrics.DryRunDispatches); got != 0 {
		t.Errorf("flux_dispatch_dryrun_dispatches_total series = %d, want 0", got)
	}
}

// TestDryRunStillRejectsNonAltinnRepo is TestDryRunStillRejectsMissingRepo's
// companion for the other rejection a product team can trigger.
func TestDryRunStillRejectsNonAltinnRepo(t *testing.T) {
	h := newDryRunHarness(t)

	body := buildEvent(eventOptions{
		reason:       "ReconciliationSucceeded",
		revision:     "at23@sha256:aabbccdd",
		product:      "dialogporten",
		env:          "at23",
		dispatchRepo: "Evil/repo",
	})
	rec := h.post(body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := h.github.count(); got != 0 {
		t.Errorf("github dispatch calls = %d, want 0", got)
	}
	if got := testutil.CollectAndCount(h.metrics.DryRunDispatches); got != 0 {
		t.Errorf("flux_dispatch_dryrun_dispatches_total series = %d, want 0", got)
	}
}

// TestDryRunLogsStructuredFields pins the exact field set the dry-run log
// line carries: outcome=dry_run plus repo, event_type, product, env, reason,
// commit_sha, revision, kustomization_name and the truncated message.
func TestDryRunLogsStructuredFields(t *testing.T) {
	h := newDryRunHarness(t)

	rec := h.post(successEvent())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	record := findLogRecord(t, h.logs.String(), "dry_run")
	for field, want := range map[string]any{
		"product":            "dialogporten",
		"env":                "at23",
		"repo":               "Altinn/dialogporten",
		"reason":             "ReconciliationSucceeded",
		"event_type":         "flux-deploy",
		"outcome":            "dry_run",
		"commit_sha":         "abc1234def5678",
		"revision":           "at23@sha256:aabbccdd",
		"kustomization_name": "dialogporten-apps",
		"message":            "Applied revision at23@sha256:aabbccdd",
	} {
		if got := record[field]; got != want {
			t.Errorf("log field %q = %v, want %v", field, got, want)
		}
	}
}

// TestDryRunLongMessageIsTruncatedInLog matches TestLongMessageIsTruncated:
// the dry-run log must carry the same truncated message a real dispatch
// would have sent, not the raw Flux message.
func TestDryRunLongMessageIsTruncatedInLog(t *testing.T) {
	h := newDryRunHarness(t)

	body := buildEvent(eventOptions{
		reason:        "ReconciliationFailed",
		revision:      "at23@sha256:aabbccdd",
		product:       "dialogporten",
		env:           "at23",
		dispatchRepo:  "Altinn/dialogporten",
		dispatchEvent: "flux-deploy-failed",
		message:       strings.Repeat("e", 8000),
	})
	if rec := h.post(body); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	record := findLogRecord(t, h.logs.String(), "dry_run")
	message, _ := record["message"].(string)
	if len(message) != 1024 {
		t.Errorf("logged message length = %d, want 1024", len(message))
	}
}

// findLogRecord returns the first JSON log line whose outcome field matches
// wantOutcome, failing the test if none is found.
func findLogRecord(t *testing.T, logs, wantOutcome string) map[string]any {
	t.Helper()
	var record map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(logs), "\n") {
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if record["outcome"] == wantOutcome {
			return record
		}
	}
	t.Fatalf("no log record with outcome=%s; logs:\n%s", wantOutcome, logs)
	return nil
}
