package config

import (
	"strings"
	"testing"
	"time"
)

// requiredEnv is the minimal set of variables that must be present for Load to
// succeed. Each test starts from this set and then removes or overrides.
func requiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_APP_ID", "123456")
	t.Setenv("GITHUB_INSTALLATION_ID", "7891011")
	t.Setenv("GITHUB_PRIVATE_KEY_PATH", "/etc/flux-dispatch/github-app.pem")
}

func TestLoadDefaults(t *testing.T) {
	requiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.GitHubAppID != "123456" {
		t.Errorf("GitHubAppID = %q, want %q", cfg.GitHubAppID, "123456")
	}
	if cfg.GitHubInstallationID != "7891011" {
		t.Errorf("GitHubInstallationID = %q, want %q", cfg.GitHubInstallationID, "7891011")
	}
	if cfg.GitHubPrivateKeyPath != "/etc/flux-dispatch/github-app.pem" {
		t.Errorf("GitHubPrivateKeyPath = %q", cfg.GitHubPrivateKeyPath)
	}
	if cfg.GitHubAPIURL != "https://api.github.com" {
		t.Errorf("GitHubAPIURL = %q, want %q", cfg.GitHubAPIURL, "https://api.github.com")
	}
	if cfg.DedupTTL != 24*time.Hour {
		t.Errorf("DedupTTL = %v, want %v", cfg.DedupTTL, 24*time.Hour)
	}
	if cfg.DedupMaxEntries != 10000 {
		t.Errorf("DedupMaxEntries = %d, want %d", cfg.DedupMaxEntries, 10000)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":8080")
	}
	if cfg.MetricsAddr != ":9090" {
		t.Errorf("MetricsAddr = %q, want %q", cfg.MetricsAddr, ":9090")
	}
	if cfg.DefaultDispatchEvent != "flux-deploy" {
		t.Errorf("DefaultDispatchEvent = %q, want %q", cfg.DefaultDispatchEvent, "flux-deploy")
	}
}

func TestLoadOverrides(t *testing.T) {
	requiredEnv(t)
	t.Setenv("GITHUB_API_URL", "https://github.example.com/api/v3")
	t.Setenv("DEDUP_TTL", "45m")
	t.Setenv("DEDUP_MAX_ENTRIES", "25")
	t.Setenv("LISTEN_ADDR", "127.0.0.1:18080")
	t.Setenv("METRICS_ADDR", "127.0.0.1:19090")
	t.Setenv("DEFAULT_DISPATCH_EVENT", "custom-deploy")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.GitHubAPIURL != "https://github.example.com/api/v3" {
		t.Errorf("GitHubAPIURL = %q", cfg.GitHubAPIURL)
	}
	if cfg.DedupTTL != 45*time.Minute {
		t.Errorf("DedupTTL = %v, want %v", cfg.DedupTTL, 45*time.Minute)
	}
	if cfg.DedupMaxEntries != 25 {
		t.Errorf("DedupMaxEntries = %d, want 25", cfg.DedupMaxEntries)
	}
	if cfg.ListenAddr != "127.0.0.1:18080" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.MetricsAddr != "127.0.0.1:19090" {
		t.Errorf("MetricsAddr = %q", cfg.MetricsAddr)
	}
	if cfg.DefaultDispatchEvent != "custom-deploy" {
		t.Errorf("DefaultDispatchEvent = %q", cfg.DefaultDispatchEvent)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	for _, name := range []string{
		"GITHUB_APP_ID",
		"GITHUB_INSTALLATION_ID",
		"GITHUB_PRIVATE_KEY_PATH",
	} {
		t.Run(name, func(t *testing.T) {
			requiredEnv(t)
			t.Setenv(name, "")

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() with %s unset returned no error", name)
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error %q does not name the missing variable %q", err, name)
			}
		})
	}
}

func TestLoadInvalidDedupTTL(t *testing.T) {
	requiredEnv(t)
	t.Setenv("DEDUP_TTL", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() with invalid DEDUP_TTL returned no error")
	}
	if !strings.Contains(err.Error(), "DEDUP_TTL") {
		t.Errorf("error %q does not name DEDUP_TTL", err)
	}
}

func TestLoadInvalidDedupMaxEntries(t *testing.T) {
	for _, value := range []string{"not-a-number", "0", "-5"} {
		t.Run(value, func(t *testing.T) {
			requiredEnv(t)
			t.Setenv("DEDUP_MAX_ENTRIES", value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() with DEDUP_MAX_ENTRIES=%q returned no error", value)
			}
			if !strings.Contains(err.Error(), "DEDUP_MAX_ENTRIES") {
				t.Errorf("error %q does not name DEDUP_MAX_ENTRIES", err)
			}
		})
	}
}
