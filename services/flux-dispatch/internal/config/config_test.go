package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// requiredEnv is the minimal set of variables that must be present for Load to
// succeed. Each test starts from this set and then removes or overrides. It
// returns the GitHub App private key path it wired up: the startup
// readability check (see TestLoadPrivateKeyMustExistAndBeReadable) requires
// that path to point at a real file, so the fixture can no longer be a
// placeholder string.
func requiredEnv(t *testing.T) string {
	t.Helper()
	t.Setenv("GITHUB_APP_ID", "123456")
	t.Setenv("GITHUB_INSTALLATION_ID", "7891011")

	keyPath := writableKeyFile(t)
	t.Setenv("GITHUB_PRIVATE_KEY_PATH", keyPath)
	return keyPath
}

// writableKeyFile returns the path to a temp file that exists and is
// readable, standing in for the GitHub App private key mounted in production.
func writableKeyFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "github-app.pem")
	if err := os.WriteFile(path, []byte("test private key"), 0o600); err != nil {
		t.Fatalf("write test private key at %s: %v", path, err)
	}
	return path
}

func TestLoadDefaults(t *testing.T) {
	keyPath := requiredEnv(t)

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
	if cfg.GitHubPrivateKeyPath != keyPath {
		t.Errorf("GitHubPrivateKeyPath = %q, want %q", cfg.GitHubPrivateKeyPath, keyPath)
	}
	if cfg.DryRun {
		t.Error("DryRun = true, want false (DRY_RUN unset)")
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

// TestLoadDryRunParsing covers DECISION-dry-run.md "Config": DRY_RUN parses
// with strconv.ParseBool, so both "true" and "1" (and their false-y
// counterparts) are accepted.
func TestLoadDryRunParsing(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			requiredEnv(t)
			t.Setenv("DRY_RUN", tt.value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() returned error: %v", err)
			}
			if cfg.DryRun != tt.want {
				t.Errorf("DryRun = %v, want %v", cfg.DryRun, tt.want)
			}
		})
	}
}

func TestLoadInvalidDryRun(t *testing.T) {
	requiredEnv(t)
	t.Setenv("DRY_RUN", "not-a-bool")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() with invalid DRY_RUN returned no error")
	}
	if !strings.Contains(err.Error(), "DRY_RUN") {
		t.Errorf("error %q does not name DRY_RUN", err)
	}
}

// TestLoadDryRunMakesGitHubVarsOptional is the point of the mode: no GitHub
// App, no private key, no Key Vault secret needed for the first deploy.
func TestLoadDryRunMakesGitHubVarsOptional(t *testing.T) {
	t.Setenv("DRY_RUN", "true")
	t.Setenv("GITHUB_APP_ID", "")
	t.Setenv("GITHUB_INSTALLATION_ID", "")
	t.Setenv("GITHUB_PRIVATE_KEY_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with DRY_RUN=true and no GitHub vars returned error: %v", err)
	}
	if cfg.GitHubAppID != "" {
		t.Errorf("GitHubAppID = %q, want empty", cfg.GitHubAppID)
	}
	if cfg.GitHubInstallationID != "" {
		t.Errorf("GitHubInstallationID = %q, want empty", cfg.GitHubInstallationID)
	}
	if cfg.GitHubPrivateKeyPath != "" {
		t.Errorf("GitHubPrivateKeyPath = %q, want empty", cfg.GitHubPrivateKeyPath)
	}
}

// TestLoadDryRunFalseStillRequiresGitHubVars is TestLoadMissingRequired's
// companion with DRY_RUN=false set explicitly rather than left unset, so the
// two ways of being "not in dry run" are both proven to still require the
// GitHub vars.
func TestLoadDryRunFalseStillRequiresGitHubVars(t *testing.T) {
	requiredEnv(t)
	t.Setenv("DRY_RUN", "false")
	t.Setenv("GITHUB_APP_ID", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() with DRY_RUN=false and GITHUB_APP_ID unset returned no error")
	}
	if !strings.Contains(err.Error(), "GITHUB_APP_ID") {
		t.Errorf("error %q does not name GITHUB_APP_ID", err)
	}
}

// TestLoadPrivateKeyMustExistAndBeReadable covers the startup check from
// DECISION-dry-run.md "Config": when DRY_RUN=false, the private key file must
// exist and be readable at Load time, not just wired as a non-empty string —
// so a bad mount fails the pod at startup instead of on the first webhook.
func TestLoadPrivateKeyMustExistAndBeReadable(t *testing.T) {
	t.Run("nonexistent", func(t *testing.T) {
		requiredEnv(t)
		t.Setenv("GITHUB_PRIVATE_KEY_PATH", filepath.Join(t.TempDir(), "does-not-exist.pem"))

		_, err := Load()
		if err == nil {
			t.Fatal("Load() with a nonexistent GITHUB_PRIVATE_KEY_PATH returned no error")
		}
		if !strings.Contains(err.Error(), "GITHUB_PRIVATE_KEY_PATH") {
			t.Errorf("error %q does not name GITHUB_PRIVATE_KEY_PATH", err)
		}
	})

	t.Run("unreadable", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("running as root can read any file regardless of its mode")
		}
		requiredEnv(t)
		path := filepath.Join(t.TempDir(), "unreadable.pem")
		if err := os.WriteFile(path, []byte("secret"), 0o000); err != nil {
			t.Fatalf("write unreadable key file: %v", err)
		}
		t.Setenv("GITHUB_PRIVATE_KEY_PATH", path)

		_, err := Load()
		if err == nil {
			t.Fatal("Load() with an unreadable GITHUB_PRIVATE_KEY_PATH returned no error")
		}
		if !strings.Contains(err.Error(), "GITHUB_PRIVATE_KEY_PATH") {
			t.Errorf("error %q does not name GITHUB_PRIVATE_KEY_PATH", err)
		}
	})
}

// TestLoadDryRunSkipsPrivateKeyReadabilityCheck is the other half of
// DECISION-dry-run.md's startup-check paragraph: DRY_RUN=true does not care
// whether the key file exists at all.
func TestLoadDryRunSkipsPrivateKeyReadabilityCheck(t *testing.T) {
	t.Setenv("DRY_RUN", "true")
	t.Setenv("GITHUB_APP_ID", "")
	t.Setenv("GITHUB_INSTALLATION_ID", "")
	t.Setenv("GITHUB_PRIVATE_KEY_PATH", filepath.Join(t.TempDir(), "does-not-exist.pem"))

	if _, err := Load(); err != nil {
		t.Fatalf("Load() with DRY_RUN=true and a nonexistent key path returned error: %v", err)
	}
}
