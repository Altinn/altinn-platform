package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

// staticToken is a TokenSource that always hands out the same token.
type staticToken string

func (s staticToken) Token(context.Context) (string, error) { return string(s), nil }

// failingToken is a TokenSource that always fails, standing in for a GitHub
// App auth outage.
type failingToken struct{}

func (failingToken) Token(context.Context) (string, error) {
	return "", errors.New("no installation token for you")
}

// capturedRequest holds what the fake GitHub saw.
type capturedRequest struct {
	path        string
	method      string
	auth        string
	accept      string
	contentType string
	body        map[string]any
}

// fakeGitHub records the last dispatch request and answers with status.
type fakeGitHub struct {
	server *httptest.Server
	hits   atomic.Int64
	last   atomic.Value // capturedRequest
	status int
}

func newFakeGitHub(t *testing.T, status int) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{status: status}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)

		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)

		f.last.Store(capturedRequest{
			path:        r.URL.Path,
			method:      r.Method,
			auth:        r.Header.Get("Authorization"),
			accept:      r.Header.Get("Accept"),
			contentType: r.Header.Get("Content-Type"),
			body:        body,
		})

		w.WriteHeader(f.status)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGitHub) request(t *testing.T) capturedRequest {
	t.Helper()
	value, ok := f.last.Load().(capturedRequest)
	if !ok {
		t.Fatal("fake GitHub received no request")
	}
	return value
}

func samplePayload() Payload {
	return Payload{
		Product:           "dialogporten",
		Environment:       "at23",
		CommitSHA:         "abc1234def5678",
		Revision:          "at23@sha256:aabbccdd",
		KustomizationName: "dialogporten-apps",
		Reason:            "ReconciliationSucceeded",
		Message:           "Applied revision at23@sha256:aabbccdd",
	}
}

func TestSendSuccess(t *testing.T) {
	fake := newFakeGitHub(t, http.StatusNoContent)
	d := New(fake.server.URL, staticToken("ghs_installation"), fake.server.Client())

	if err := d.Send(context.Background(), "Altinn/dialogporten", "flux-deploy", samplePayload()); err != nil {
		t.Fatalf("Send() returned error: %v", err)
	}

	req := fake.request(t)
	if req.path != "/repos/Altinn/dialogporten/dispatches" {
		t.Errorf("path = %q, want %q", req.path, "/repos/Altinn/dialogporten/dispatches")
	}
	if req.method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.method)
	}
	if req.auth != "Bearer ghs_installation" {
		t.Errorf("Authorization = %q", req.auth)
	}
	if req.accept != "application/vnd.github+json" {
		t.Errorf("Accept = %q, want application/vnd.github+json", req.accept)
	}
	if !strings.HasPrefix(req.contentType, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", req.contentType)
	}

	if got := req.body["event_type"]; got != "flux-deploy" {
		t.Errorf("event_type = %v, want flux-deploy", got)
	}

	clientPayload, ok := req.body["client_payload"].(map[string]any)
	if !ok {
		t.Fatalf("client_payload is not an object: %#v", req.body["client_payload"])
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
		if got := clientPayload[field]; got != want {
			t.Errorf("client_payload[%q] = %v, want %q", field, got, want)
		}
	}
}

func TestSendTruncatesMessage(t *testing.T) {
	fake := newFakeGitHub(t, http.StatusNoContent)
	d := New(fake.server.URL, staticToken("ghs"), fake.server.Client())

	p := samplePayload()
	p.Message = strings.Repeat("e", 4096)

	if err := d.Send(context.Background(), "Altinn/dialogporten", "flux-deploy-failed", p); err != nil {
		t.Fatalf("Send() returned error: %v", err)
	}

	clientPayload := fake.request(t).body["client_payload"].(map[string]any)
	message, _ := clientPayload["message"].(string)
	if len(message) != maxMessageLen {
		t.Errorf("message length = %d, want %d", len(message), maxMessageLen)
	}

	// The caller's Payload must not be mutated by the truncation.
	if len(p.Message) != 4096 {
		t.Errorf("Send() mutated the caller's payload: message length = %d", len(p.Message))
	}
}

func TestSendTruncatesOnRuneBoundary(t *testing.T) {
	fake := newFakeGitHub(t, http.StatusNoContent)
	d := New(fake.server.URL, staticToken("ghs"), fake.server.Client())

	p := samplePayload()
	p.Message = strings.Repeat("æ", 2000) // 2 bytes per rune

	if err := d.Send(context.Background(), "Altinn/dialogporten", "flux-deploy-failed", p); err != nil {
		t.Fatalf("Send() returned error: %v", err)
	}

	clientPayload := fake.request(t).body["client_payload"].(map[string]any)
	message, _ := clientPayload["message"].(string)
	if !utf8.ValidString(message) {
		t.Error("truncated message is not valid UTF-8")
	}
	if utf8.RuneCountInString(message) > maxMessageLen {
		t.Errorf("message rune count = %d, want <= %d", utf8.RuneCountInString(message), maxMessageLen)
	}
}

func TestSendShortMessageUntouched(t *testing.T) {
	fake := newFakeGitHub(t, http.StatusNoContent)
	d := New(fake.server.URL, staticToken("ghs"), fake.server.Client())

	if err := d.Send(context.Background(), "Altinn/dialogporten", "flux-deploy", samplePayload()); err != nil {
		t.Fatalf("Send() returned error: %v", err)
	}

	clientPayload := fake.request(t).body["client_payload"].(map[string]any)
	if got := clientPayload["message"]; got != "Applied revision at23@sha256:aabbccdd" {
		t.Errorf("message = %v, want it untouched", got)
	}
}

func TestSendErrorClassification(t *testing.T) {
	tests := []struct {
		status   int
		wantErr  error
		wantCode string
	}{
		{http.StatusNotFound, ErrNonRetryable, "404"},
		{http.StatusUnauthorized, ErrNonRetryable, "401"},
		{http.StatusForbidden, ErrNonRetryable, "403"},
		{http.StatusUnprocessableEntity, ErrNonRetryable, "422"},
		{http.StatusInternalServerError, ErrRetryable, "500"},
		{http.StatusBadGateway, ErrRetryable, "502"},
		{http.StatusServiceUnavailable, ErrRetryable, "503"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.status), func(t *testing.T) {
			fake := newFakeGitHub(t, tt.status)
			d := New(fake.server.URL, staticToken("ghs"), fake.server.Client())

			err := d.Send(context.Background(), "Altinn/dialogporten", "flux-deploy", samplePayload())
			if err == nil {
				t.Fatalf("Send() with status %d returned no error", tt.status)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Send() error = %v, want it to wrap %v", err, tt.wantErr)
			}
			if got := ErrorCode(err); got != tt.wantCode {
				t.Errorf("ErrorCode() = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestSendTimeoutIsRetryable(t *testing.T) {
	// release lets the handler return at cleanup time; without it the server's
	// Close would block on the in-flight request.
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	// Cleanups run LIFO: unblock the handler first, then close the server.
	t.Cleanup(slow.Close)
	t.Cleanup(func() { close(release) })

	d := New(slow.URL, staticToken("ghs"), slow.Client())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := d.Send(ctx, "Altinn/dialogporten", "flux-deploy", samplePayload())
	if err == nil {
		t.Fatal("Send() against a hanging server returned no error")
	}
	if !errors.Is(err, ErrRetryable) {
		t.Errorf("Send() error = %v, want it to wrap ErrRetryable", err)
	}
	if got := ErrorCode(err); got != "timeout" {
		t.Errorf("ErrorCode() = %q, want %q", got, "timeout")
	}
}

func TestSendTokenFailureIsRetryable(t *testing.T) {
	fake := newFakeGitHub(t, http.StatusNoContent)
	d := New(fake.server.URL, failingToken{}, fake.server.Client())

	err := d.Send(context.Background(), "Altinn/dialogporten", "flux-deploy", samplePayload())
	if err == nil {
		t.Fatal("Send() with a failing token source returned no error")
	}
	if !errors.Is(err, ErrAuth) {
		t.Errorf("Send() error = %v, want it to wrap ErrAuth", err)
	}
	if !errors.Is(err, ErrRetryable) {
		t.Errorf("Send() error = %v, want it to wrap ErrRetryable", err)
	}
	if fake.hits.Load() != 0 {
		t.Errorf("endpoint hits = %d, want 0 (must not call GitHub without a token)", fake.hits.Load())
	}
}

func TestSendRejectsRepoThatWouldEscapeThePath(t *testing.T) {
	fake := newFakeGitHub(t, http.StatusNoContent)
	d := New(fake.server.URL, staticToken("ghs"), fake.server.Client())

	for _, repo := range []string{"Altinn/../../secret", "Altinn/..", "Altinn/.", "Altinn/...", "../x"} {
		err := d.Send(context.Background(), repo, "flux-deploy", samplePayload())
		if err == nil {
			t.Fatalf("Send(%q) returned no error", repo)
		}
		if !errors.Is(err, ErrNonRetryable) {
			t.Errorf("Send(%q) error = %v, want it to wrap ErrNonRetryable", repo, err)
		}
	}
	if fake.hits.Load() != 0 {
		t.Errorf("endpoint hits = %d, want 0 (no request may leave with a traversal repo)", fake.hits.Load())
	}
}

// TestTruncateMessageMatchesSendTruncation pins TruncateMessage to the same
// limit Send applies to an outbound payload's message field, so the server's
// dry-run logging path (which never calls Send) can still log the message a
// real dispatch would have carried.
func TestTruncateMessageMatchesSendTruncation(t *testing.T) {
	short := "Applied revision at23@sha256:aabbccdd"
	if got := TruncateMessage(short); got != short {
		t.Errorf("TruncateMessage(%q) = %q, want it untouched", short, got)
	}

	long := strings.Repeat("e", 4096)
	got := TruncateMessage(long)
	if len(got) != maxMessageLen {
		t.Errorf("TruncateMessage(long) length = %d, want %d", len(got), maxMessageLen)
	}
	if !utf8.ValidString(got) {
		t.Error("TruncateMessage did not preserve valid UTF-8")
	}
}

func TestErrorCodeOnUnrelatedError(t *testing.T) {
	if got := ErrorCode(errors.New("boom")); got != "" {
		t.Errorf("ErrorCode(unrelated) = %q, want empty", got)
	}
	if got := ErrorCode(nil); got != "" {
		t.Errorf("ErrorCode(nil) = %q, want empty", got)
	}
}

// rotatingToken is a TokenSource whose token GitHub can revoke, so a test can
// observe whether Send drops a rejected one instead of presenting it again.
type rotatingToken struct {
	issued      atomic.Int64
	invalidated atomic.Value // string
	current     atomic.Value // string
}

func newRotatingToken() *rotatingToken {
	r := &rotatingToken{}
	r.mint()
	return r
}

func (r *rotatingToken) mint() {
	r.current.Store(fmt.Sprintf("ghs_token_%d", r.issued.Add(1)))
}

func (r *rotatingToken) Token(context.Context) (string, error) {
	token, _ := r.current.Load().(string)
	return token, nil
}

func (r *rotatingToken) InvalidateToken(token string) {
	r.invalidated.Store(token)
	r.mint()
}

// permanentToken stands in for a GitHub App that will never authenticate: a
// malformed key, or an App or installation ID GitHub rejects.
type permanentToken struct{}

func (permanentToken) Token(context.Context) (string, error) { return "", permanentTokenErr{} }

type permanentTokenErr struct{}

func (permanentTokenErr) Error() string   { return "app credentials rejected" }
func (permanentTokenErr) Permanent() bool { return true }

// TestSendRetriesOnceAfterUnauthorized covers a token GitHub revoked mid-life,
// which is what an App key rotation looks like from here.
func TestSendRetriesOnceAfterUnauthorized(t *testing.T) {
	var calls atomic.Int64
	var seen []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	tokens := newRotatingToken()
	dispatcher := New(server.URL, tokens, server.Client())

	if err := dispatcher.Send(context.Background(), "Altinn/repo", "flux-deploy", Payload{}); err != nil {
		t.Fatalf("Send() error = %v, want nil after the retry succeeded", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("dispatch attempts = %d, want 2", got)
	}
	if got, _ := tokens.invalidated.Load().(string); got != "ghs_token_1" {
		t.Errorf("invalidated token = %q, want %q", got, "ghs_token_1")
	}
	if seen[0] == seen[1] {
		t.Errorf("retry reused the rejected token %q", seen[0])
	}
}

// TestSendDoesNotRetryTwice keeps the refresh from becoming a retry loop: if a
// freshly minted token is also rejected, the failure stands.
func TestSendDoesNotRetryTwice(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	dispatcher := New(server.URL, newRotatingToken(), server.Client())

	err := dispatcher.Send(context.Background(), "Altinn/repo", "flux-deploy", Payload{})
	if err == nil {
		t.Fatal("Send() error = nil, want an error when both attempts are rejected")
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("dispatch attempts = %d, want exactly 2", got)
	}
	if !errors.Is(err, ErrNonRetryable) {
		t.Errorf("error = %v, want ErrNonRetryable for a persistent 401", err)
	}
}

// TestSendAuthErrorClassification pins the split the return-code contract rests
// on: a rejected credential must not be answered 502.
func TestSendAuthErrorClassification(t *testing.T) {
	tests := []struct {
		name        string
		tokens      TokenSource
		wantClass   error
		wantCode    string
		wantOtherIs error
	}{
		{
			name:        "transient outage",
			tokens:      failingToken{},
			wantClass:   ErrRetryable,
			wantCode:    "auth",
			wantOtherIs: ErrAuth,
		},
		{
			name:        "rejected credentials",
			tokens:      permanentToken{},
			wantClass:   ErrNonRetryable,
			wantCode:    "auth_permanent",
			wantOtherIs: ErrAuth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispatcher := New("https://api.github.invalid", tt.tokens, http.DefaultClient)

			err := dispatcher.Send(context.Background(), "Altinn/repo", "flux-deploy", Payload{})
			if err == nil {
				t.Fatal("Send() error = nil, want an auth error")
			}
			if !errors.Is(err, tt.wantClass) {
				t.Errorf("error = %v, want it to wrap %v", err, tt.wantClass)
			}
			if !errors.Is(err, tt.wantOtherIs) {
				t.Errorf("error = %v, want it to wrap %v", err, tt.wantOtherIs)
			}
			if got := ErrorCode(err); got != tt.wantCode {
				t.Errorf("ErrorCode() = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

// TestErrorMessage covers the Error string, which lands in the operator-facing
// log line for every failed dispatch.
func TestErrorMessage(t *testing.T) {
	err := &Error{Code: "429", Status: 429, Class: ErrRetryable, Reason: "rate limited"}
	if got, want := err.Error(), "dispatch failed (429): rate limited"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
