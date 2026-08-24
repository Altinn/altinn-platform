package validate

import (
	"strings"
	"testing"
)

func TestRepoAllowed(t *testing.T) {
	tests := []struct {
		name    string
		repo    string
		wantErr bool
	}{
		{"canonical repo", "Altinn/dialogporten", false},
		{"dots and dashes", "Altinn/altinn-platform.core_v2", false},
		{"other org rejected", "Evil/repo", true},
		{"lowercase org rejected", "altinn/dialogporten", true},
		{"org prefix substring rejected", "AltinnEvil/repo", true},
		{"three segments rejected", "Altinn/a/b", true},
		{"path traversal rejected", "Altinn/../x", true},
		// Dot-only segments are well-formed to repoRe (it allows "." in the
		// name class) and pass the org prefix check, but url.JoinPath *cleans*
		// them instead of rejecting them, so a value that survives validation
		// silently rewrites the outbound request path.
		{"parent traversal rejected", "Altinn/..", true},
		{"current dir rejected", "Altinn/.", true},
		{"triple dot rejected", "Altinn/...", true},
		{"many dots rejected", "Altinn/....", true},
		{"dot-only owner rejected", "../dialogporten", true},
		{"dot-only owner and name rejected", "../..", true},
		// Leading and trailing dots are legitimate GitHub repo names; only an
		// all-dot segment is a traversal token, so these must still pass.
		{"dotfile repo name allowed", "Altinn/.github", false},
		{"trailing dot name allowed", "Altinn/repo.", false},
		{"empty rejected", "", true},
		{"owner only rejected", "Altinn", true},
		{"trailing slash rejected", "Altinn/", true},
		{"leading slash rejected", "/Altinn/dialogporten", true},
		{"space rejected", "Altinn/dialog porten", true},
		{"url escape rejected", "Altinn/%2e%2e", true},
		{"newline rejected", "Altinn/repo\n", true},
		{"query injection rejected", "Altinn/repo?x=1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RepoAllowed(tt.repo)
			if tt.wantErr && err == nil {
				t.Fatalf("RepoAllowed(%q) = nil, want error", tt.repo)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("RepoAllowed(%q) = %v, want nil", tt.repo, err)
			}
		})
	}
}

func TestKnownReason(t *testing.T) {
	// Every reason the service accepts must be reported as known.
	for reason := range successReasons {
		if !KnownReason(reason) {
			t.Errorf("KnownReason(%q) = false, want true", reason)
		}
	}
	for reason := range failureReasons {
		if !KnownReason(reason) {
			t.Errorf("KnownReason(%q) = false, want true", reason)
		}
	}

	for _, reason := range []string{
		"", "Progressing", "ProgressingWithRetry", "info", "reconciliationsucceeded",
		"HealthCheckCanceled", "InvalidCELExpression", "unknown",
	} {
		if KnownReason(reason) {
			t.Errorf("KnownReason(%q) = true, want false", reason)
		}
	}
}

// TestReasonSets pins the exact accepted reason set from RFC 0010. These are
// kustomize-controller v1 event reasons (fluxcd/pkg/apis/meta).
func TestReasonSets(t *testing.T) {
	wantSuccess := []string{"ReconciliationSucceeded"}
	wantFailure := []string{
		"ReconciliationFailed", "BuildFailed", "HealthCheckFailed",
		"PruneFailed", "DependencyNotReady", "ArtifactFailed",
	}

	if len(successReasons) != len(wantSuccess) {
		t.Errorf("successReasons has %d entries, want %d", len(successReasons), len(wantSuccess))
	}
	for _, reason := range wantSuccess {
		if !successReasons[reason] {
			t.Errorf("successReasons is missing %q", reason)
		}
	}

	if len(failureReasons) != len(wantFailure) {
		t.Errorf("failureReasons has %d entries, want %d", len(failureReasons), len(wantFailure))
	}
	for _, reason := range wantFailure {
		if !failureReasons[reason] {
			t.Errorf("failureReasons is missing %q", reason)
		}
	}

	for reason := range successReasons {
		if failureReasons[reason] {
			t.Errorf("%q is in both successReasons and failureReasons", reason)
		}
	}
}

func TestShouldDispatch(t *testing.T) {
	tests := []struct {
		reason        string
		dispatchEvent string
		want          bool
	}{
		{"ReconciliationSucceeded", "flux-deploy", true},
		{"ReconciliationFailed", "flux-deploy", false},
		{"ReconciliationFailed", "flux-deploy-failed", true},
		{"BuildFailed", "flux-deploy-failed", true},
		{"HealthCheckFailed", "flux-deploy", false},
		{"ReconciliationSucceeded", "flux-deploy-failed", false},
		{"HealthCheckFailed", "flux-deploy-failed", true},
		{"PruneFailed", "flux-deploy-failed", true},
		{"DependencyNotReady", "flux-deploy-failed", true},
		{"ArtifactFailed", "flux-deploy-failed", true},
		// Any non "-failed" event type is a success channel.
		{"ReconciliationSucceeded", "run-e2e", true},
		{"ReconciliationFailed", "run-e2e", false},
		{"ReconciliationFailed", "incident-failed", true},
		// Unknown reasons are never dispatched, on either channel.
		{"Progressing", "flux-deploy", false},
		{"Progressing", "flux-deploy-failed", false},
		{"", "flux-deploy", false},
		{"", "flux-deploy-failed", false},
	}

	for _, tt := range tests {
		t.Run(tt.reason+"/"+tt.dispatchEvent, func(t *testing.T) {
			if got := ShouldDispatch(tt.reason, tt.dispatchEvent); got != tt.want {
				t.Errorf("ShouldDispatch(%q, %q) = %v, want %v", tt.reason, tt.dispatchEvent, got, tt.want)
			}
		})
	}
}

func TestReasonLabel(t *testing.T) {
	tests := []struct{ reason, want string }{
		{"ReconciliationSucceeded", "ReconciliationSucceeded"},
		{"ReconciliationFailed", "ReconciliationFailed"},
		{"HealthCheckFailed", "HealthCheckFailed"},
		// Anything unrecognised shares one bucket, so a hostile or misconfigured
		// Alert cannot mint an unbounded number of metric series.
		{"SomethingBrandNew", OtherReasonLabel},
		{"", OtherReasonLabel},
		{"../../etc/passwd", OtherReasonLabel},
	}

	for _, tt := range tests {
		if got := ReasonLabel(tt.reason); got != tt.want {
			t.Errorf("ReasonLabel(%q) = %q, want %q", tt.reason, got, tt.want)
		}
	}
}

func TestDispatchEvent(t *testing.T) {
	tests := []struct {
		name          string
		dispatchEvent string
		wantErr       bool
	}{
		{"typical", "flux-deploy", false},
		{"failure channel", "flux-deploy-failed", false},
		{"dots and underscores", "flux_deploy.v2", false},
		{"at the length limit", strings.Repeat("a", 100), false},

		{"empty", "", true},
		{"over the length limit", strings.Repeat("a", 101), true},
		{"newline", "flux-deploy\nX-Injected: 1", true},
		{"space", "flux deploy", true},
		{"slash", "flux/deploy", true},
		{"quote", `flux"deploy`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DispatchEvent(tt.dispatchEvent)
			if (err != nil) != tt.wantErr {
				t.Errorf("DispatchEvent(%q) error = %v, wantErr %v", tt.dispatchEvent, err, tt.wantErr)
			}
		})
	}
}

func TestRepoAllowedRejectsOverlongNames(t *testing.T) {
	tests := []struct {
		name    string
		repo    string
		wantErr bool
	}{
		{"normal", "Altinn/dialogporten", false},
		{"at the repo name limit", "Altinn/" + strings.Repeat("a", 100), false},
		{"over the repo name limit", "Altinn/" + strings.Repeat("a", 101), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RepoAllowed(tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("RepoAllowed(%q) error = %v, wantErr %v", tt.repo, err, tt.wantErr)
			}
		})
	}
}
