// Package event models the webhook body Flux's notification-controller posts
// and the handful of derived values flux-dispatch needs from it: the source
// commit SHA, the OCI revision and its digest, and the Alert's eventMetadata.
package event

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Metadata keys set by kustomize-controller on every Kustomization event.
const (
	// originRevisionKey holds "{branch}/{commit-sha}" of the source the artifact
	// was built from — set by product CI via `flux push artifact --source`.
	originRevisionKey = "kustomize.toolkit.fluxcd.io/originRevision"
	// revisionKey holds the reconciled artifact revision, e.g. "at23@sha256:aabbccdd".
	revisionKey = "kustomize.toolkit.fluxcd.io/revision"
)

// Metadata keys product teams set through the Flux Alert's eventMetadata.
const (
	MetaProduct       = "product"
	MetaEnv           = "env"
	MetaDispatchRepo  = "dispatch_repo"
	MetaDispatchEvent = "dispatch_event"
)

// FluxEvent is the subset of the notification-controller event payload the
// service consumes. Unknown fields are ignored so new Flux releases adding
// fields do not break parsing.
type FluxEvent struct {
	InvolvedObject struct {
		Kind, Name, Namespace string
	} `json:"involvedObject"`
	Severity  string            `json:"severity"`
	Reason    string            `json:"reason"`
	Message   string            `json:"message"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata"`
}

// Parse decodes a webhook body into a FluxEvent. Only a body that is not
// decodable JSON is an error.
//
// There are deliberately no field-presence checks: an empty reason is just an
// unrecognised reason, and RFC 0010 step 3 answers 200 OK for those. Failing
// the parse instead would push cases the RFC mandates a 200 for behind an
// error response.
func Parse(r io.Reader) (FluxEvent, error) {
	var e FluxEvent
	if err := json.NewDecoder(r).Decode(&e); err != nil {
		// Wrapped, not replaced: the caller unwraps to spot
		// *http.MaxBytesError and answer 413.
		return FluxEvent{}, fmt.Errorf("decode flux event: %w", err)
	}
	return e, nil
}

// CommitSHA returns the source commit SHA carried in originRevision
// ("main/abc123" -> "abc123"). Branch names may contain "/", so the SHA is the
// segment after the last separator. It returns "" when the key is absent.
func (e FluxEvent) CommitSHA() string {
	origin := e.Metadata[originRevisionKey]
	if origin == "" {
		return ""
	}
	if idx := strings.LastIndex(origin, "/"); idx >= 0 {
		return origin[idx+1:]
	}
	return origin
}

// Revision returns the full reconciled revision, e.g. "at23@sha256:aabbccdd".
func (e FluxEvent) Revision() string {
	return e.Metadata[revisionKey]
}

// Digest returns the digest part of the revision — "at23@sha256:aabbccdd"
// becomes "sha256:aabbccdd", a bare "sha256:…" is passed through. This is the
// value the dedup key is built from: it changes exactly when the artifact does.
func (e FluxEvent) Digest() string {
	revision := e.Revision()
	if revision == "" {
		return ""
	}
	if idx := strings.LastIndex(revision, "@"); idx >= 0 {
		return revision[idx+1:]
	}
	return revision
}

// Meta returns the Alert eventMetadata value for key, or "" when absent.
func (e FluxEvent) Meta(key string) string {
	return e.Metadata[key]
}
