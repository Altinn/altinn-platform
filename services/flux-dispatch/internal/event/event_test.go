package event

import (
	"strings"
	"testing"
	"time"
)

// samplePayload is the webhook body from the Flux reconcile-webhooks design doc
// (§"Webhook Payload"), copied verbatim including its `{placeholder}` values.
// Parsing it exactly as written proves the struct tags match what Flux's
// notification-controller puts on the wire.
const samplePayload = `{
  "involvedObject": {
    "kind": "Kustomization",
    "name": "my-app-kustomization",
    "namespace": "product-{name}"
  },
  "severity": "info",
  "reason": "ReconciliationSucceeded",
  "metadata": {
    "kustomize.toolkit.fluxcd.io/originRevision": "main/abc1234def5678",
    "kustomize.toolkit.fluxcd.io/revision": "{env}@sha256:...",
    "product": "{name}",
    "env": "{environment}"
  },
  "timestamp": "2026-03-05T12:00:00Z"
}`

// realisticPayload is the same shape with the concrete values used throughout
// RFC 0010, plus the `message` and dispatch routing metadata a real Alert sends.
const realisticPayload = `{
  "involvedObject": {
    "kind": "Kustomization",
    "name": "dialogporten-apps",
    "namespace": "product-dialogporten"
  },
  "severity": "info",
  "reason": "ReconciliationSucceeded",
  "message": "Applied revision at23@sha256:aabbccdd",
  "metadata": {
    "kustomize.toolkit.fluxcd.io/originRevision": "main/abc1234def5678",
    "kustomize.toolkit.fluxcd.io/revision": "at23@sha256:aabbccdd",
    "product": "dialogporten",
    "env": "at23",
    "dispatch_repo": "Altinn/dialogporten",
    "dispatch_event": "flux-deploy"
  },
  "timestamp": "2026-03-05T12:00:00Z"
}`

func TestParseSamplePayload(t *testing.T) {
	e, err := Parse(strings.NewReader(samplePayload))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if e.InvolvedObject.Kind != "Kustomization" {
		t.Errorf("InvolvedObject.Kind = %q, want %q", e.InvolvedObject.Kind, "Kustomization")
	}
	if e.InvolvedObject.Name != "my-app-kustomization" {
		t.Errorf("InvolvedObject.Name = %q", e.InvolvedObject.Name)
	}
	if e.InvolvedObject.Namespace != "product-{name}" {
		t.Errorf("InvolvedObject.Namespace = %q", e.InvolvedObject.Namespace)
	}
	if e.Severity != "info" {
		t.Errorf("Severity = %q", e.Severity)
	}
	if e.Reason != "ReconciliationSucceeded" {
		t.Errorf("Reason = %q", e.Reason)
	}
	want := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	if !e.Timestamp.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", e.Timestamp, want)
	}
	if got := e.CommitSHA(); got != "abc1234def5678" {
		t.Errorf("CommitSHA() = %q, want %q", got, "abc1234def5678")
	}
	if got := e.Meta("product"); got != "{name}" {
		t.Errorf("Meta(product) = %q", got)
	}
	if got := e.Meta("env"); got != "{environment}" {
		t.Errorf("Meta(env) = %q", got)
	}
}

func TestParseRealisticPayload(t *testing.T) {
	e, err := Parse(strings.NewReader(realisticPayload))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if e.InvolvedObject.Name != "dialogporten-apps" {
		t.Errorf("InvolvedObject.Name = %q", e.InvolvedObject.Name)
	}
	if e.Message != "Applied revision at23@sha256:aabbccdd" {
		t.Errorf("Message = %q", e.Message)
	}
	if got := e.Revision(); got != "at23@sha256:aabbccdd" {
		t.Errorf("Revision() = %q", got)
	}
	if got := e.Digest(); got != "sha256:aabbccdd" {
		t.Errorf("Digest() = %q", got)
	}
	if got := e.CommitSHA(); got != "abc1234def5678" {
		t.Errorf("CommitSHA() = %q", got)
	}
	if got := e.Meta(MetaDispatchRepo); got != "Altinn/dialogporten" {
		t.Errorf("Meta(dispatch_repo) = %q", got)
	}
	if got := e.Meta(MetaDispatchEvent); got != "flux-deploy" {
		t.Errorf("Meta(dispatch_event) = %q", got)
	}
}

func TestParseIgnoresUnknownFields(t *testing.T) {
	const payload = `{
  "involvedObject": {"kind":"Kustomization","name":"app","namespace":"ns","apiVersion":"kustomize.toolkit.fluxcd.io/v1","uid":"1234"},
  "severity": "info",
  "reason": "ReconciliationSucceeded",
  "message": "Applied revision",
  "reportingController": "kustomize-controller",
  "reportingInstance": "kustomize-controller-abc",
  "timestamp": "2026-03-05T12:00:00Z",
  "metadata": {"product":"p"},
  "someFutureField": {"nested": [1,2,3]}
}`

	e, err := Parse(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if e.Reason != "ReconciliationSucceeded" {
		t.Errorf("Reason = %q", e.Reason)
	}
	if e.InvolvedObject.Name != "app" {
		t.Errorf("InvolvedObject.Name = %q", e.InvolvedObject.Name)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"not json", "this is not json"},
		{"empty body", ""},
		{"json array", `["nope"]`},
		{"wrong type for reason", `{"reason":{"nested":true}}`},
		{"wrong type for metadata", `{"reason":"X","metadata":"not-an-object"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(tt.body)); err == nil {
				t.Fatalf("Parse(%q) returned no error", tt.body)
			}
		})
	}
}

// TestParseAcceptsSparseEvents pins M2: an event that is valid JSON but missing
// reason or involvedObject fields must parse cleanly, so the handler can answer
// 200 per RFC step 3 instead of hiding it behind an error response.
func TestParseAcceptsSparseEvents(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing reason", `{"involvedObject":{"kind":"Kustomization","name":"app"},"severity":"info"}`},
		{"missing involvedObject kind", `{"involvedObject":{"name":"app"},"reason":"ReconciliationSucceeded"}`},
		{"missing involvedObject name", `{"involvedObject":{"kind":"Kustomization"},"reason":"ReconciliationSucceeded"}`},
		{"empty object", `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(tt.body)); err != nil {
				t.Fatalf("Parse(%q) returned error %v, want nil", tt.body, err)
			}
		})
	}
}

func TestCommitSHA(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		want     string
	}{
		{"branch and sha", map[string]string{originRevisionKey: "main/abc1234def5678"}, "abc1234def5678"},
		{"branch with slashes", map[string]string{originRevisionKey: "feature/x/abc123"}, "abc123"},
		{"no separator", map[string]string{originRevisionKey: "abc123"}, "abc123"},
		{"missing key", map[string]string{"env": "at23"}, ""},
		{"nil metadata", nil, ""},
		{"empty value", map[string]string{originRevisionKey: ""}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := FluxEvent{Metadata: tt.metadata}
			if got := e.CommitSHA(); got != tt.want {
				t.Errorf("CommitSHA() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRevision(t *testing.T) {
	e := FluxEvent{Metadata: map[string]string{revisionKey: "at23@sha256:aabbccdd"}}
	if got := e.Revision(); got != "at23@sha256:aabbccdd" {
		t.Errorf("Revision() = %q", got)
	}
	if got := (FluxEvent{}).Revision(); got != "" {
		t.Errorf("Revision() on empty event = %q, want empty", got)
	}
}

func TestDigest(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		want     string
	}{
		{"tag and digest", map[string]string{revisionKey: "at23@sha256:aabbccdd"}, "sha256:aabbccdd"},
		{"bare digest", map[string]string{revisionKey: "sha256:aabbccdd"}, "sha256:aabbccdd"},
		{"missing key", map[string]string{"env": "at23"}, ""},
		{"nil metadata", nil, ""},
		{"empty value", map[string]string{revisionKey: ""}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := FluxEvent{Metadata: tt.metadata}
			if got := e.Digest(); got != tt.want {
				t.Errorf("Digest() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMeta(t *testing.T) {
	e := FluxEvent{Metadata: map[string]string{
		MetaProduct:       "dialogporten",
		MetaEnv:           "at23",
		MetaDispatchRepo:  "Altinn/dialogporten",
		MetaDispatchEvent: "flux-deploy-failed",
	}}

	for key, want := range map[string]string{
		MetaProduct:       "dialogporten",
		MetaEnv:           "at23",
		MetaDispatchRepo:  "Altinn/dialogporten",
		MetaDispatchEvent: "flux-deploy-failed",
		"absent":          "",
	} {
		if got := e.Meta(key); got != want {
			t.Errorf("Meta(%q) = %q, want %q", key, got, want)
		}
	}

	if got := (FluxEvent{}).Meta(MetaProduct); got != "" {
		t.Errorf("Meta on nil metadata = %q, want empty", got)
	}
}
