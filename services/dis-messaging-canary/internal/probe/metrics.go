// Package probe measures the round trip of the canary: send one message to a
// queue, receive it and complete it.
package probe

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metric names. Dashboards, alert rules and SLOs read them, so they all live
// in this file.
const (
	roundTripsName  = "dis_messaging_canary_round_trips_total"
	failuresName    = "dis_messaging_canary_round_trip_failures_total"
	durationName    = "dis_messaging_canary_round_trip_duration_seconds"
	lastSuccessName = "dis_messaging_canary_last_success_timestamp_seconds"
	staleName       = "dis_messaging_canary_stale_messages_total"
	stageLabel      = "stage"
)

// Stages of a round trip. They are the values of the stage label.
const (
	stageSend     = "send"
	stageReceive  = "receive"
	stageComplete = "complete"
)

// Metrics holds the metrics of the canary.
type Metrics struct {
	roundTrips  prometheus.Counter
	failures    *prometheus.CounterVec
	duration    prometheus.Histogram
	lastSuccess prometheus.Gauge
	stale       prometheus.Counter
}

// NewMetrics creates the metrics and registers them on reg.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)
	m := &Metrics{
		roundTrips: factory.NewCounter(prometheus.CounterOpts{
			Name: roundTripsName, Help: "Round trips that ended, with or without a failure.",
		}),
		failures: factory.NewCounterVec(prometheus.CounterOpts{
			Name: failuresName, Help: "Round trips that failed, by the stage that failed.",
		}, []string{stageLabel}),
		duration: factory.NewHistogram(prometheus.HistogramOpts{
			Name: durationName, Help: "Time from send to complete of the round trips that succeeded.",
			Buckets: []float64{0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20},
		}),
		lastSuccess: factory.NewGauge(prometheus.GaugeOpts{
			Name: lastSuccessName, Help: "Unix time of the last round trip that succeeded. 0 before the first one.",
		}),
		stale: factory.NewCounter(prometheus.CounterOpts{
			Name: staleName, Help: "Messages from earlier round trips that the canary removed from the queue.",
		}),
	}
	// Every failure series must exist at 0 from the start. A failure ratio
	// breaks when the failure series exists only during an incident.
	for _, stage := range []string{stageSend, stageReceive, stageComplete} {
		m.failures.WithLabelValues(stage)
	}
	return m
}
