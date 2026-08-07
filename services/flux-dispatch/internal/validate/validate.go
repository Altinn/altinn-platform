// Package validate holds the service's admission rules: which Flux
// reconciliation reasons are acted on, which dispatch targets are allowed, and
// how reasons are routed to success- versus failure-type dispatch events.
package validate

import (
	"fmt"
	"regexp"
	"strings"
)

// successReasons are the kustomize-controller v1 event reasons that mean the
// Kustomization reconciled successfully.
var successReasons = map[string]bool{"ReconciliationSucceeded": true}

// failureReasons are the kustomize-controller v1 event reasons that mean the
// reconciliation failed. Verified against fluxcd/pkg/apis/meta (the constants
// kustomize-controller emits): ReconciliationFailedReason, BuildFailedReason,
// HealthCheckFailedReason, PruneFailedReason, DependencyNotReadyReason and
// ArtifactFailedReason. "HealthCheckFailed" is only emitted for Kustomizations
// with wait/healthChecks configured.
var failureReasons = map[string]bool{
	"ReconciliationFailed": true, "BuildFailed": true, "HealthCheckFailed": true,
	"PruneFailed": true, "DependencyNotReady": true, "ArtifactFailed": true,
}

// repoRe is deliberately strict: exactly one "/" and no path or query
// characters, so the value can never escape the /repos/{repo}/dispatches path.
var repoRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)

// orgPrefix is the only GitHub organisation the service will dispatch to.
const orgPrefix = "Altinn/"

// failedEventSuffix marks a dispatch_event as a failure channel.
const failedEventSuffix = "-failed"

// KnownReason reports whether reason is a reconciliation event the service
// acts on. Anything else is acknowledged and ignored.
func KnownReason(reason string) bool {
	return successReasons[reason] || failureReasons[reason]
}

// RepoAllowed checks a product-supplied dispatch_repo: strict owner/repo form
// and the Altinn org prefix, rejecting cross-org dispatch attempts.
func RepoAllowed(repo string) error {
	if repo == "" {
		return fmt.Errorf("dispatch_repo is empty")
	}
	if !repoRe.MatchString(repo) {
		return fmt.Errorf("dispatch_repo %q is not in owner/repo format", repo)
	}
	// repoRe allows "." inside a segment, so "Altinn/.." is well-formed to it.
	// url.JoinPath *cleans* traversal segments rather than rejecting them, so a
	// value that survived validation would silently rewrite the outbound path.
	// Reject all-dot segments; leading/trailing dots (".github", "repo.") stay
	// legal because they are real GitHub repository names.
	owner, name, _ := strings.Cut(repo, "/")
	for _, segment := range []string{owner, name} {
		if strings.Trim(segment, ".") == "" {
			return fmt.Errorf("dispatch_repo %q contains a path-traversal segment", repo)
		}
	}
	if !strings.HasPrefix(repo, orgPrefix) {
		return fmt.Errorf("dispatch_repo %q is not in the %s organisation", repo, strings.TrimSuffix(orgPrefix, "/"))
	}
	return nil
}

// ShouldDispatch decides whether an event with this reason belongs on this
// dispatch_event channel. Flux's `eventSeverity: info` forwards error events
// too, so the routing is done here by reason: a dispatch_event ending in
// "-failed" carries failure reasons only, any other value carries success
// reasons only.
func ShouldDispatch(reason, dispatchEvent string) bool {
	if strings.HasSuffix(dispatchEvent, failedEventSuffix) {
		return failureReasons[reason]
	}
	return successReasons[reason]
}
