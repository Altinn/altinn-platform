// Package server wires the request flow from RFC 0010 into one HTTP handler:
// parse, validate, deduplicate, dispatch. The return codes follow the RFC's
// contract — 2xx whenever retrying cannot help, 502 only for transient GitHub
// failures, 413 for an oversized body. Access control is enforced entirely by
// the NetworkPolicy restricting ingress on this port to flux-system.
package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/dedup"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/dispatch"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/event"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/metrics"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/validate"
)

// maxBodyBytes is the request-body limit from RFC 0010.
const maxBodyBytes = 64 * 1024

// Timeouts from RFC 0010 §"HTTP server hardening".
const (
	readTimeout       = 10 * time.Second
	readHeaderTimeout = 5 * time.Second
	writeTimeout      = 30 * time.Second
	maxHeaderBytes    = 1 << 16 // 64 KB
)

// Outcome values used in the structured logs and as the handler's summary of
// what happened to an event.
const (
	outcomeDispatched          = "dispatched"
	outcomeDuplicate           = "duplicate"
	outcomeNoDigest            = "no_digest"
	outcomeIgnoredReason       = "ignored_reason"
	outcomeNotRouted           = "reason_not_routed"
	outcomeMissingRepo         = "missing_dispatch_repo"
	outcomeInvalidRepo         = "invalid_dispatch_repo"
	outcomeBodyTooLarge        = "body_too_large"
	outcomeInvalidPayload      = "invalid_payload"
	outcomeInvalidEvent        = "invalid_dispatch_event"
	outcomeDispatchRejected    = "dispatch_rejected"
	outcomeDispatchUnavailable = "dispatch_unavailable"
	outcomeAuthFailed          = "github_auth_failed"
	outcomeAuthRejected        = "github_auth_rejected"
	outcomeDryRun              = "dry_run"
)

// dispatchBudget bounds the whole outbound conversation with GitHub — a token
// exchange plus the dispatch itself, each of which can take githubTimeout. It
// sits below writeTimeout so an unresponsive GitHub surfaces as a 502 the
// caller actually receives, rather than a dropped connection.
const dispatchBudget = 25 * time.Second

// Options configures a Server. Tracker, Dispatcher and Metrics are required for
// the webhook handler; MetricsHandler and the *Server accessors work without.
type Options struct {
	ListenAddr           string
	MetricsAddr          string
	DefaultDispatchEvent string
	// DryRun, when true, runs the handler through every step exactly as
	// normal and then logs the dispatch it would have sent instead of
	// calling GitHub. See README.md "Configuration".
	DryRun     bool
	Tracker    *dedup.Tracker
	Dispatcher *dispatch.Dispatcher
	Metrics    *metrics.Metrics
	Logger     *slog.Logger
}

// Server serves the webhook endpoint, the health endpoints and the metrics
// registry.
type Server struct {
	opts Options
	log  *slog.Logger
}

// New returns a Server for opts.
func New(opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{opts: opts, log: logger}
}

// Handler returns the mux served on the webhook port.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /flux-events", s.handleFluxEvent)
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /readyz", handleHealth)
	return mux
}

// MetricsHandler returns the mux served on the metrics port.
func (s *Server) MetricsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", s.opts.Metrics.Handler())
	return mux
}

// WebhookServer returns the hardened HTTP server for the webhook port.
func (s *Server) WebhookServer() *http.Server {
	return &http.Server{
		Addr:              s.opts.ListenAddr,
		Handler:           s.Handler(),
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
}

// MetricsServer returns the HTTP server for the metrics port.
func (s *Server) MetricsServer() *http.Server {
	return &http.Server{
		Addr:              s.opts.MetricsAddr,
		Handler:           s.MetricsHandler(),
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
}

// handleHealth answers the liveness and readiness probes. The service holds no
// external state that could make it unready after start-up.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
}

// handleFluxEvent implements RFC 0010 §"Request flow", steps 1-10 in order.
// When Options.DryRun is true, every step still runs; only the final GitHub
// call (steps 8-10) is replaced with a log line.
func (s *Server) handleFluxEvent(w http.ResponseWriter, r *http.Request) {
	// Cap the body before anything reads it. This must stay first: it bounds
	// the read itself, so nothing downstream can be handed an unbounded body.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			s.log.Warn("webhook body exceeds limit",
				"outcome", outcomeBodyTooLarge, "limit_bytes", maxBodyBytes)
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		s.log.Warn("reading webhook body failed",
			"outcome", outcomeInvalidPayload, "error", err)
		writeAccepted(w, outcomeInvalidPayload)
		return
	}

	// Step 2: parse.
	e, err := event.Parse(bytes.NewReader(body))
	if err != nil {
		// Non-retryable, so 2xx per the return-code contract: Flux collapses
		// every non-2xx into one "failed to send notification" class, so a 4xx
		// here would be indistinguishable from a real outage on its side. The
		// diagnostic signal lives in this warning log instead.
		s.log.Warn("rejecting unparseable webhook body",
			"outcome", outcomeInvalidPayload, "error", err)
		writeAccepted(w, outcomeInvalidPayload)
		return
	}

	product := e.Meta(event.MetaProduct)
	env := e.Meta(event.MetaEnv)
	repo := e.Meta(event.MetaDispatchRepo)
	log := s.log.With(
		"product", product,
		"env", env,
		"repo", repo,
		"reason", e.Reason,
		"kustomization", e.InvolvedObject.Name,
	)

	// Bucket the label rather than passing the raw reason through: it arrives
	// verbatim in the request body, and a Prometheus CounterVec never evicts a
	// child, so an unfiltered value would pin a series for the process lifetime.
	s.opts.Metrics.EventsReceived.WithLabelValues(validate.ReasonLabel(e.Reason)).Inc()

	// Step 3: only act on recognised reconciliation reasons.
	if !validate.KnownReason(e.Reason) {
		log.Debug("ignoring event with unrecognised reason", "outcome", outcomeIgnoredReason)
		writeAccepted(w, outcomeIgnoredReason)
		return
	}

	// Step 4: the Alert must name a dispatch target.
	if repo == "" {
		log.Warn("event has no dispatch_repo in eventMetadata", "outcome", outcomeMissingRepo)
		writeAccepted(w, outcomeMissingRepo)
		return
	}

	// Step 5: the target must be a well-formed repo in the Altinn org.
	if err := validate.RepoAllowed(repo); err != nil {
		log.Warn("rejecting dispatch target", "outcome", outcomeInvalidRepo, "error", err)
		writeAccepted(w, outcomeInvalidRepo)
		return
	}

	eventType := e.Meta(event.MetaDispatchEvent)
	if eventType == "" {
		eventType = s.opts.DefaultDispatchEvent
	}

	// The dispatch_event reaches GitHub and three Prometheus labels, so it is
	// bounded here the same way the dispatch_repo is.
	if err := validate.DispatchEvent(eventType); err != nil {
		log.Warn("rejecting dispatch_event", "outcome", outcomeInvalidEvent, "error", err)
		writeAccepted(w, outcomeInvalidEvent)
		return
	}
	log = log.With("event_type", eventType)

	// Route by reason: an `eventSeverity: info` Alert also forwards errors, and
	// those must not go out as a success event (and vice versa).
	if !validate.ShouldDispatch(e.Reason, eventType) {
		log.Debug("reason does not belong on this dispatch_event", "outcome", outcomeNotRouted)
		writeAccepted(w, outcomeNotRouted)
		return
	}

	// Step 7: skip events already dispatched for this artifact.
	//
	// The digest is the only part of the key that identifies the artifact. An
	// event without one — anything not carrying kustomize-controller's revision
	// metadata — would collapse onto a single key per product/env/reason/repo,
	// so the first delivery would suppress every later one for the whole TTL.
	// Dispatching without dedup risks a duplicate workflow run; deduplicating
	// on an empty digest silently stops deploys. The former is recoverable.
	digest := e.Digest()
	var dedupKey string
	dispatched := false

	if digest == "" {
		log.Warn("event carries no artifact digest, dispatching without deduplication",
			"outcome", outcomeNoDigest)
	} else {
		dedupKey = s.opts.Tracker.Key(product, env, e.Reason, digest, repo)

		// Claim is atomic. Checking Seen here and recording after the dispatch
		// would leave the whole outbound call as a window in which a second
		// delivery of the same event also sees "unseen" and dispatches too.
		if !s.opts.Tracker.Claim(dedupKey) {
			s.opts.Metrics.DedupHits.WithLabelValues(e.Reason).Inc()
			log.Info("skipping duplicate event", "outcome", outcomeDuplicate, "revision", e.Revision())
			writeAccepted(w, outcomeDuplicate)
			return
		}

		// The claim suppresses duplicates only until this handler resolves it,
		// so every path out of here must either confirm it (success) or release
		// it. Deferred so a panic cannot strand the key for the whole TTL.
		defer func() {
			if !dispatched {
				s.opts.Tracker.Release(dedupKey)
			}
		}()
	}

	// recordDispatch confirms the claim, so the event is deduplicated from here
	// on. It is a no-op when dedup was skipped for a missing digest.
	recordDispatch := func() {
		if dedupKey == "" {
			return
		}
		s.opts.Tracker.Record(dedupKey)
		dispatched = true
	}

	// Steps 6, 8, 9, 10: build the URL, extract the commit SHA, authenticate as
	// the App and post the dispatch.
	payload := dispatch.Payload{
		Product:           product,
		Environment:       env,
		CommitSHA:         e.CommitSHA(),
		Revision:          e.Revision(),
		KustomizationName: e.InvolvedObject.Name,
		Reason:            e.Reason,
		Message:           e.Message,
	}

	if s.opts.DryRun {
		// Dedup key recorded here too: observing dedup behaviour is a main
		// purpose of dry-run. See README.md "DRY_RUN mode".
		recordDispatch()
		s.opts.Metrics.DryRunDispatches.WithLabelValues(repo, eventType, e.Reason).Inc()
		log.Info("dry run: would have dispatched repository_dispatch",
			"outcome", outcomeDryRun,
			"commit_sha", payload.CommitSHA,
			"revision", payload.Revision,
			"kustomization_name", payload.KustomizationName,
			"message", dispatch.TruncateMessage(payload.Message))
		writeAccepted(w, outcomeDryRun)
		return
	}

	// Bound the token exchange and the dispatch together, so the pair cannot
	// outlast the server's write timeout and drop the response.
	dispatchCtx, cancel := context.WithTimeout(r.Context(), dispatchBudget)
	defer cancel()

	start := time.Now()
	dispatchErr := s.opts.Dispatcher.Send(dispatchCtx, repo, eventType, payload)
	elapsed := time.Since(start)

	if errors.Is(dispatchErr, dispatch.ErrAuth) {
		// No dispatch was attempted, so there is no latency to record —
		// observing here would poison the histogram. It must still land in
		// dispatch_errors_total: without that, a rotated App key fails 100% of
		// dispatches while any error-rate alert reads a flat zero.
		s.opts.Metrics.GitHubAuthErrors.Inc()
		s.opts.Metrics.DispatchErrors.WithLabelValues(repo, eventType, dispatch.ErrorCode(dispatchErr)).Inc()

		if errors.Is(dispatchErr, dispatch.ErrRetryable) {
			log.Error("could not authenticate as the GitHub App",
				"outcome", outcomeAuthFailed, "error", dispatchErr)
			http.Error(w, "github authentication unavailable", http.StatusBadGateway)
			return
		}

		// Permanent: a malformed key, or an App or installation ID GitHub
		// rejects outright. No retry can fix it, so acknowledge instead of
		// making Flux exhaust its retries and drop the event anyway. The signal
		// lives in this log line and in github_auth_errors_total.
		log.Error("github rejected the App credentials, retrying cannot help",
			"outcome", outcomeAuthRejected, "error", dispatchErr)
		writeAccepted(w, outcomeAuthRejected)
		return
	}

	s.opts.Metrics.DispatchDuration.WithLabelValues(repo).Observe(elapsed.Seconds())

	switch {
	case dispatchErr == nil:
		recordDispatch()
		s.opts.Metrics.Dispatches.WithLabelValues(repo, eventType, e.Reason).Inc()
		log.Info("dispatched repository_dispatch",
			"outcome", outcomeDispatched,
			"commit_sha", payload.CommitSHA,
			"revision", payload.Revision,
			"duration_ms", elapsed.Milliseconds())
		writeAccepted(w, outcomeDispatched)

	case errors.Is(dispatchErr, dispatch.ErrRetryable):
		s.opts.Metrics.DispatchErrors.WithLabelValues(repo, eventType, dispatch.ErrorCode(dispatchErr)).Inc()
		log.Error("dispatch failed, asking Flux to retry",
			"outcome", outcomeDispatchUnavailable, "error", dispatchErr)
		http.Error(w, "github dispatch unavailable", http.StatusBadGateway)

	default:
		// Non-retryable: the App is probably not installed on the target repo.
		s.opts.Metrics.DispatchErrors.WithLabelValues(repo, eventType, dispatch.ErrorCode(dispatchErr)).Inc()
		log.Warn("dispatch rejected by GitHub",
			"outcome", outcomeDispatchRejected, "error", dispatchErr)
		writeAccepted(w, outcomeDispatchRejected)
	}
}

// writeAccepted answers 200 with the outcome, the code the service returns for
// everything Flux must not retry.
func writeAccepted(w http.ResponseWriter, outcome string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, outcome+"\n")
}
