// Package config loads the flux-dispatch runtime configuration from the
// environment. Required values have no defaults and Load fails loudly, naming
// the missing variable, so a misconfigured Deployment crash-loops instead of
// silently serving requests it cannot dispatch.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds every knob the service reads at startup.
type Config struct {
	// GitHubAppID is the numeric App ID of the GitHub App used to authenticate
	// outbound repository_dispatch calls.
	GitHubAppID string
	// GitHubInstallationID identifies the App installation whose access token
	// the service exchanges its JWT for.
	GitHubInstallationID string
	// GitHubPrivateKeyPath points at the PEM-encoded RSA private key of the App
	// (mounted from a Kubernetes Secret).
	GitHubPrivateKeyPath string
	// GitHubAPIURL is the API base, overridable for GitHub Enterprise and tests.
	GitHubAPIURL string
	// DedupTTL is how long a dispatched event key is remembered.
	DedupTTL time.Duration
	// DedupMaxEntries caps the dedup tracker; the oldest entry is evicted at cap.
	DedupMaxEntries int
	// ListenAddr is the webhook listener address.
	ListenAddr string
	// MetricsAddr is the Prometheus listener address.
	MetricsAddr string
	// DefaultDispatchEvent is used when an Alert omits dispatch_event.
	DefaultDispatchEvent string
}

// Defaults applied when the corresponding environment variable is unset.
const (
	defaultGitHubAPIURL         = "https://api.github.com"
	defaultDedupTTL             = 24 * time.Hour
	defaultDedupMaxEntries      = 10000
	defaultListenAddr           = ":8080"
	defaultMetricsAddr          = ":9090"
	defaultDefaultDispatchEvent = "flux-deploy"
)

// Load reads the configuration from the process environment, applying defaults
// for every optional value. It returns an error naming the offending variable
// when a required value is missing or an optional value cannot be parsed.
func Load() (Config, error) {
	cfg := Config{
		GitHubAPIURL:         envOr("GITHUB_API_URL", defaultGitHubAPIURL),
		DedupTTL:             defaultDedupTTL,
		DedupMaxEntries:      defaultDedupMaxEntries,
		ListenAddr:           envOr("LISTEN_ADDR", defaultListenAddr),
		MetricsAddr:          envOr("METRICS_ADDR", defaultMetricsAddr),
		DefaultDispatchEvent: envOr("DEFAULT_DISPATCH_EVENT", defaultDefaultDispatchEvent),
	}

	for _, required := range []struct {
		name string
		dst  *string
	}{
		{"GITHUB_APP_ID", &cfg.GitHubAppID},
		{"GITHUB_INSTALLATION_ID", &cfg.GitHubInstallationID},
		{"GITHUB_PRIVATE_KEY_PATH", &cfg.GitHubPrivateKeyPath},
	} {
		value := os.Getenv(required.name)
		if value == "" {
			return Config{}, fmt.Errorf("required environment variable %s is not set", required.name)
		}
		*required.dst = value
	}

	if raw := os.Getenv("DEDUP_TTL"); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DEDUP_TTL %q: %w", raw, err)
		}
		if ttl <= 0 {
			return Config{}, fmt.Errorf("invalid DEDUP_TTL %q: must be positive", raw)
		}
		cfg.DedupTTL = ttl
	}

	if raw := os.Getenv("DEDUP_MAX_ENTRIES"); raw != "" {
		maxEntries, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid DEDUP_MAX_ENTRIES %q: %w", raw, err)
		}
		if maxEntries <= 0 {
			return Config{}, fmt.Errorf("invalid DEDUP_MAX_ENTRIES %q: must be positive", raw)
		}
		cfg.DedupMaxEntries = maxEntries
	}

	return cfg, nil
}

// envOr returns the value of name, or fallback when it is unset or empty.
func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
