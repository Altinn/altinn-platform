package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

// DurationComponents holds the individual components of a duration
type DurationComponents struct {
	Years        int
	Weeks        int
	Days         int
	Hours        int
	Minutes      int
	Seconds      int
	Milliseconds int
}

// durationRegex matches Prometheus duration strings
var durationRegex = regexp.MustCompile(`^(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?$`)

// ParsePromDuration parses a Prometheus duration string into DurationComponents
func ParsePromDuration(str string) (*DurationComponents, error) {
	matches := durationRegex.FindStringSubmatch(str)
	if matches == nil || matches[0] != str {
		return nil, fmt.Errorf("invalid Prometheus duration: %s", str)
	}

	var dur DurationComponents

	// Helper function to parse a matched duration component
	parseComponent := func(index int) int {
		if matches[index] != "" {
			value, _ := strconv.Atoi(matches[index])
			return value
		}
		return 0
	}

	dur.Years = parseComponent(2)
	dur.Weeks = parseComponent(4)
	dur.Days = parseComponent(6)
	dur.Hours = parseComponent(8)
	dur.Minutes = parseComponent(10)
	dur.Seconds = parseComponent(12)
	dur.Milliseconds = parseComponent(14)

	return &dur, nil
}

// PromDurationToISO8601 converts a Prometheus duration string to an ISO 8601 duration string
func PromDurationToISO8601(promDuration string) (string, error) {
	dur, err := ParsePromDuration(promDuration)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("P")

	// Date components
	if dur.Years > 0 {
		builder.WriteString(fmt.Sprintf("%dY", dur.Years))
	}
	if dur.Weeks > 0 {
		builder.WriteString(fmt.Sprintf("%dW", dur.Weeks))
	}
	if dur.Days > 0 {
		builder.WriteString(fmt.Sprintf("%dD", dur.Days))
	}

	// Time components
	if dur.Hours > 0 || dur.Minutes > 0 || dur.Seconds > 0 || dur.Milliseconds > 0 {
		builder.WriteString("T")
	}

	if dur.Hours > 0 {
		builder.WriteString(fmt.Sprintf("%dH", dur.Hours))
	}
	if dur.Minutes > 0 {
		builder.WriteString(fmt.Sprintf("%dM", dur.Minutes))
	}

	if dur.Seconds > 0 || dur.Milliseconds > 0 {
		// Handle fractional seconds
		totalSeconds := float64(dur.Seconds) + float64(dur.Milliseconds)/1000.0
		// Remove trailing zeros and dot if not needed
		secondsStr := strconv.FormatFloat(totalSeconds, 'f', -1, 64)
		builder.WriteString(fmt.Sprintf("%sS", secondsStr))
	}

	isoDuration := builder.String()
	return isoDuration, nil
}

// Helper function to create a *monitoringv1.Duration from a string
func NewDuration(duration string) *monitoringv1.Duration {
	d := monitoringv1.Duration(duration)
	return &d
}
