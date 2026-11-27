package probe

import (
	"maps"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func failures(stage string) string { return failuresName + "{" + stage + "}" }

// zero is the value of every series before the first round trip.
func zero() map[string]float64 {
	return map[string]float64{
		roundTripsName: 0, durationName: 0, lastSuccessName: 0, staleName: 0,
		failures(stageSend): 0, failures(stageReceive): 0, failures(stageComplete): 0,
	}
}

// values reads every series of reg. The key is the metric name, plus the stage
// when the series has one. The value of a histogram is its number of samples.
func values(t *testing.T, reg *prometheus.Registry) map[string]float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, family := range families {
		for _, m := range family.GetMetric() {
			key := family.GetName()
			if labels := m.GetLabel(); len(labels) > 0 {
				key += "{" + labels[0].GetValue() + "}"
			}
			switch {
			case m.GetHistogram() != nil:
				got[key] = float64(m.GetHistogram().GetSampleCount())
			case m.GetGauge() != nil:
				got[key] = m.GetGauge().GetValue()
			default:
				got[key] = m.GetCounter().GetValue()
			}
		}
	}
	return got
}

func TestEverySeriesExistsAtZeroFromStart(t *testing.T) {
	reg := prometheus.NewRegistry()
	NewMetrics(reg)
	if got := values(t, reg); !maps.Equal(got, zero()) {
		t.Fatalf("metrics at start:\n got %v\nwant %v", got, zero())
	}
}
