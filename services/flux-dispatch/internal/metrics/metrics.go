// Package metrics defines the Prometheus collectors from RFC 0010. They live on
// a dedicated registry served on the metrics port, so the webhook port exposes
// nothing but the handler and health endpoints.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the eight collectors from RFC 0010 §"Prometheus metrics".
type Metrics struct {
	// Registry is the dedicated registry these collectors are registered on.
	Registry *prometheus.Registry

	// EventsReceived counts webhook events received from Flux, by reason.
	EventsReceived *prometheus.CounterVec
	// Dispatches counts successful repository_dispatch calls.
	Dispatches *prometheus.CounterVec
	// DispatchErrors counts failed repository_dispatch calls.
	DispatchErrors *prometheus.CounterVec
	// DedupHits counts events skipped by deduplication.
	DedupHits *prometheus.CounterVec
	// DedupEntries tracks the current size of the dedup tracker.
	DedupEntries prometheus.Gauge
	// GitHubAuthErrors counts failures obtaining an installation token.
	GitHubAuthErrors prometheus.Counter
	// DispatchDuration measures outbound repository_dispatch latency.
	DispatchDuration *prometheus.HistogramVec

	// DryRunDispatches is separate from Dispatches: the latter must never
	// move in dry-run. See README.md "Metrics".
	DryRunDispatches *prometheus.CounterVec
}

// New builds the collectors and registers them on a fresh registry.
func New() *Metrics {
	m := &Metrics{
		Registry: prometheus.NewRegistry(),

		EventsReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flux_dispatch_events_received_total",
			Help: "Total webhook events received from Flux, by reconciliation reason.",
		}, []string{"reason"}),

		Dispatches: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flux_dispatch_dispatches_total",
			Help: "Successful repository_dispatch calls to GitHub.",
		}, []string{"repo", "event_type", "reason"}),

		DispatchErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flux_dispatch_dispatch_errors_total",
			Help: "Failed repository_dispatch calls, labelled with the HTTP status or timeout.",
		}, []string{"repo", "event_type", "error_code"}),

		DedupHits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flux_dispatch_dedup_hits_total",
			Help: "Events skipped by deduplication.",
		}, []string{"reason"}),

		DedupEntries: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "flux_dispatch_dedup_entries",
			Help: "Current number of entries in the dedup tracker.",
		}),

		GitHubAuthErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "flux_dispatch_github_auth_errors_total",
			Help: "Failures obtaining a GitHub App installation token.",
		}),

		DispatchDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "flux_dispatch_dispatch_duration_seconds",
			Help:    "Latency of outbound repository_dispatch API calls.",
			Buckets: prometheus.DefBuckets,
		}, []string{"repo"}),

		DryRunDispatches: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flux_dispatch_dryrun_dispatches_total",
			Help: "Dispatches that would have been sent to GitHub, recorded while DRY_RUN is enabled.",
		}, []string{"repo", "event_type", "reason"}),
	}

	m.Registry.MustRegister(
		m.EventsReceived,
		m.Dispatches,
		m.DispatchErrors,
		m.DedupHits,
		m.DedupEntries,
		m.GitHubAuthErrors,
		m.DispatchDuration,
		m.DryRunDispatches,
	)

	return m
}

// Handler serves the registry in the Prometheus text exposition format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{Registry: m.Registry})
}
