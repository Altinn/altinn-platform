// Package config reads the settings of the canary from the environment.
package config

import (
	"cmp"
	"fmt"
	"strings"
	"time"
)

// spaces are the characters that a host name or a queue name cannot have. A
// value from a file often ends with a line break.
const spaces = " \t\r\n"

// Config holds the settings of the canary.
type Config struct {
	Endpoint    string        // host name of the Azure Service Bus namespace
	Queue       string        // name of the queue
	Interval    time.Duration // time from the start of one round trip to the next
	Timeout     time.Duration // longest time that one round trip can take
	MetricsAddr string        // listen address for /metrics and /healthz
}

// Load reads the settings with getenv, for example os.Getenv. An error names
// the variable that is wrong.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Endpoint:    getenv("ENDPOINT"),
		Queue:       getenv("QUEUE"),
		MetricsAddr: cmp.Or(getenv("CANARY_METRICS_ADDR"), ":8080"),
	}
	if cfg.Endpoint == "" || strings.ContainsAny(cfg.Endpoint, "/:"+spaces) {
		return Config{}, fmt.Errorf("ENDPOINT must be the host name of the Azure Service Bus namespace, "+
			"for example x.servicebus.windows.net, with no scheme, port or path: got %q", cfg.Endpoint)
	}
	if cfg.Queue == "" || strings.ContainsAny(cfg.Queue, spaces) {
		return Config{}, fmt.Errorf("QUEUE must be the name of the queue, with no spaces: got %q", cfg.Queue)
	}
	var err error
	if cfg.Interval, err = duration(getenv, "CANARY_INTERVAL", 30*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.Timeout, err = duration(getenv, "CANARY_TIMEOUT", 20*time.Second); err != nil {
		return Config{}, err
	}
	// The loop starts one round trip per interval. A round trip that is longer
	// than the interval delays the next one.
	if cfg.Timeout >= cfg.Interval {
		return Config{}, fmt.Errorf("CANARY_TIMEOUT (%s) must be shorter than CANARY_INTERVAL (%s)", cfg.Timeout, cfg.Interval)
	}
	return cfg, nil
}

// duration reads a duration of 1 s or more. It returns fallback when the
// variable is not set. A shorter interval would make the canary send without a
// stop, and a zero interval makes the loop panic.
func duration(getenv func(string) string, name string, fallback time.Duration) (time.Duration, error) {
	value := getenv(name)
	if value == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d < time.Second {
		return 0, fmt.Errorf("%s must be a duration of 1s or more, for example 30s: got %q", name, value)
	}
	return d, nil
}
