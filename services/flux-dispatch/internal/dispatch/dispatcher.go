// Package dispatch sends the repository_dispatch call that triggers a product's
// GitHub Actions workflow. Failures are classified so the caller can honour the
// service's return-code contract: config problems are acknowledged with 2xx,
// transient GitHub failures answer 502 so Flux retries.
package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/githubapi"
)

// maxMessageLen caps the Flux message forwarded to the workflow. GitHub limits
// client_payload size, and a kustomize build error can be very long.
const maxMessageLen = 1024

// maxErrorBody caps how much of a GitHub error response is quoted back.
const maxErrorBody = 512

// maxDrainBytes caps the successful response body read before the connection is
// returned to the pool. repository_dispatch answers 204 with no body, so this
// only needs to be comfortably larger than anything GitHub actually sends.
const maxDrainBytes = 64 * 1024

// Error classes. Send always wraps one of these so the HTTP handler can decide
// between 200 (nothing to retry) and 502 (Flux should retry).
var (
	// ErrRetryable marks a transient failure — GitHub 5xx, rate limit, timeout,
	// transport.
	ErrRetryable = errors.New("retryable dispatch failure")
	// ErrNonRetryable marks a permanent failure — GitHub 4xx, bad target repo,
	// misconfigured GitHub App.
	ErrNonRetryable = errors.New("non-retryable dispatch failure")
	// ErrAuth marks a failure to obtain a GitHub App installation token. It is
	// counted separately; whether it is retryable depends on the cause.
	ErrAuth = errors.New("github app authentication failure")
)

// Error carries the metric label for a failed dispatch alongside its class.
type Error struct {
	// Code is the flux_dispatch_dispatch_errors_total error_code label: the
	// HTTP status, "timeout", "transport", or an auth label.
	Code string
	// Status is the HTTP status GitHub returned, or 0 when the failure
	// happened before or outside an HTTP response.
	Status int
	// Class is ErrRetryable or ErrNonRetryable.
	Class error
	// Reason is the human-readable detail.
	Reason string
}

func (e *Error) Error() string {
	return fmt.Sprintf("dispatch failed (%s): %s", e.Code, e.Reason)
}

// Unwrap exposes the class (and, for auth failures, ErrAuth) to errors.Is.
func (e *Error) Unwrap() error { return e.Class }

// ErrorCode returns the metric label for err, or "" if err is not a dispatch
// error.
func ErrorCode(err error) string {
	var dispatchErr *Error
	if errors.As(err, &dispatchErr) {
		return dispatchErr.Code
	}
	return ""
}

// TokenSource provides the GitHub App installation token used to authenticate.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// TokenInvalidator is implemented by a TokenSource whose tokens GitHub can
// revoke before they expire — after a key rotation, for example. Send uses it
// to drop a token GitHub has just rejected instead of presenting it again for
// the rest of its nominal lifetime.
type TokenInvalidator interface {
	InvalidateToken(token string)
}

// permanentError is implemented by errors that know a retry cannot help. It is
// duck-typed on purpose, so this package does not depend on the token source's
// concrete implementation.
type permanentError interface {
	Permanent() bool
}

// Dispatcher posts repository_dispatch events to GitHub.
type Dispatcher struct {
	apiBase    string
	tokens     TokenSource
	httpClient *http.Client
}

// New returns a Dispatcher posting to apiBase. hc may be nil, in which case
// http.DefaultClient is used.
func New(apiBase string, tokens TokenSource, hc *http.Client) *Dispatcher {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Dispatcher{
		apiBase:    strings.TrimSuffix(apiBase, "/"),
		tokens:     tokens,
		httpClient: hc,
	}
}

// Payload is the client_payload delivered to the product's workflow.
type Payload struct {
	Product           string `json:"product"`
	Environment       string `json:"environment"`
	CommitSHA         string `json:"commit_sha"`
	Revision          string `json:"revision"`
	KustomizationName string `json:"kustomization_name"`
	Reason            string `json:"reason"`
	Message           string `json:"message"`
}

// requestBody is the repository_dispatch envelope.
type requestBody struct {
	EventType     string  `json:"event_type"`
	ClientPayload Payload `json:"client_payload"`
}

// Send posts a repository_dispatch event of type eventType to repo. It returns
// nil on GitHub 2xx, an error wrapping ErrNonRetryable on a permanent failure,
// and an error wrapping ErrRetryable on 5xx, a rate limit, a timeout, or a
// transport failure.
//
// A 401 is retried once with a freshly minted token: GitHub revokes
// installation tokens on key rotation without warning, and the cached one would
// otherwise keep failing until its nominal expiry.
func (d *Dispatcher) Send(ctx context.Context, repo, eventType string, p Payload) error {
	endpoint, err := dispatchURL(d.apiBase, repo)
	if err != nil {
		return &Error{Code: "invalid_repo", Class: ErrNonRetryable, Reason: err.Error()}
	}

	// Truncate on a copy: the caller's payload (and its log fields) stay intact.
	p.Message = truncate(p.Message, maxMessageLen)

	body, err := json.Marshal(requestBody{EventType: eventType, ClientPayload: p})
	if err != nil {
		return &Error{Code: "encode", Class: ErrNonRetryable, Reason: err.Error()}
	}

	token, err := d.tokens.Token(ctx)
	if err != nil {
		return authError(err)
	}

	sendErr := d.attempt(ctx, endpoint, token, body)
	if sendErr == nil || sendErr.Status != http.StatusUnauthorized {
		return orNil(sendErr)
	}

	invalidator, ok := d.tokens.(TokenInvalidator)
	if !ok {
		return sendErr
	}
	invalidator.InvalidateToken(token)

	token, err = d.tokens.Token(ctx)
	if err != nil {
		return authError(err)
	}
	return orNil(d.attempt(ctx, endpoint, token, body))
}

// orNil converts a typed-nil *Error into an untyped nil, so callers comparing
// against nil behave as expected.
func orNil(err *Error) error {
	if err == nil {
		return nil
	}
	return err
}

// attempt performs one repository_dispatch POST.
func (d *Dispatcher) attempt(ctx context.Context, endpoint, token string, body []byte) *Error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return &Error{Code: "request", Class: ErrNonRetryable, Reason: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return &Error{Code: transportErrorCode(err), Class: ErrRetryable, Reason: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Drain so the connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))
		return nil
	}

	detail, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	class := ErrNonRetryable
	if githubapi.Retryable(resp.StatusCode, resp.Header, detail) {
		class = ErrRetryable
	}
	return &Error{
		Code:   strconv.Itoa(resp.StatusCode),
		Status: resp.StatusCode,
		Class:  class,
		Reason: fmt.Sprintf("github returned %d: %s", resp.StatusCode, strings.TrimSpace(string(detail))),
	}
}

// authError wraps a token-source failure, preserving whether it is permanent.
// A GitHub outage is worth a retry; a malformed key or a wrong App ID is not,
// and answering 502 for those makes Flux retry until it gives up and drops the
// event.
func authError(err error) *Error {
	class := errors.Join(ErrRetryable, ErrAuth)
	code := "auth"

	var permanent permanentError
	if errors.As(err, &permanent) && permanent.Permanent() {
		class = errors.Join(ErrNonRetryable, ErrAuth)
		code = "auth_permanent"
	}

	return &Error{Code: code, Class: class, Reason: err.Error()}
}

// dispatchURL builds {apiBase}/repos/{owner}/{repo}/dispatches. The repo is
// split and joined element-wise through url.JoinPath so no "/" or ".." in the
// product-supplied value can steer the request to another endpoint.
func dispatchURL(apiBase, repo string) (string, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return "", fmt.Errorf("dispatch repo %q is not in owner/repo format", repo)
	}
	if strings.ContainsAny(owner+name, "/") {
		return "", fmt.Errorf("dispatch repo %q contains path separators", repo)
	}
	// Second layer behind validate.RepoAllowed: url.JoinPath *cleans* "." and
	// ".." rather than rejecting them, so an all-dot segment reaching here
	// would silently change the request path.
	for _, segment := range []string{owner, name} {
		if strings.Trim(segment, ".") == "" {
			return "", fmt.Errorf("dispatch repo %q contains a path-traversal segment", repo)
		}
	}

	endpoint, err := url.JoinPath(apiBase, "repos", owner, name, "dispatches")
	if err != nil {
		return "", fmt.Errorf("build dispatch URL for %q: %w", repo, err)
	}
	return endpoint, nil
}

// transportErrorCode maps a client-side failure to its metric label.
func transportErrorCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	return "transport"
}

// TruncateMessage shortens s to the same limit Send applies to an outbound
// payload's message field. It is exported so a caller that builds a Payload
// without calling Send — such as the server's DRY_RUN logging path — can log
// the same value a real dispatch would have sent.
func TruncateMessage(s string) string {
	return truncate(s, maxMessageLen)
}

// truncate shortens s to at most limit runes, never splitting a rune.
func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}
