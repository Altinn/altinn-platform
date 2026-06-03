package flux

import (
	"encoding/json"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Ready condition status values, mirroring meta.fluxcd.io condition semantics.
const (
	ReadyTrue    = "True"
	ReadyFalse   = "False"
	ReadyUnknown = "Unknown"
)

// Resource is a normalized view of a Flux custom resource's deployment status.
// Only the fields needed for the summary and "what's broken" views are pulled
// out; the full object is preserved verbatim in Raw so the detail endpoint can
// expose anything else without a schema change.
type Resource struct {
	Kind               string          `json:"kind"`
	APIVersion         string          `json:"apiVersion"`
	Namespace          string          `json:"namespace"`
	Name               string          `json:"name"`
	Ready              string          `json:"ready"` // True | False | Unknown
	Reason             string          `json:"reason,omitempty"`
	Message            string          `json:"message,omitempty"`
	Revision           string          `json:"revision,omitempty"`
	Suspended          bool            `json:"suspended"`
	Generation         int64           `json:"generation,omitempty"`
	ObservedGeneration int64           `json:"observedGeneration,omitempty"`
	LastTransition     *time.Time      `json:"lastTransition,omitempty"`
	Raw                json.RawMessage `json:"raw,omitempty"`
}

// normalize projects an unstructured Flux object into a Resource.
func normalize(u *unstructured.Unstructured) (Resource, error) {
	r := Resource{
		Kind:       u.GetKind(),
		APIVersion: u.GetAPIVersion(),
		Namespace:  u.GetNamespace(),
		Name:       u.GetName(),
		Generation: u.GetGeneration(),
		Ready:      ReadyUnknown,
	}

	obj := u.Object

	if suspended, ok, _ := unstructured.NestedBool(obj, "spec", "suspend"); ok {
		r.Suspended = suspended
	}
	if og, ok, _ := unstructured.NestedInt64(obj, "status", "observedGeneration"); ok {
		r.ObservedGeneration = og
	}

	r.applyReadyCondition(obj)
	// A stale Ready=True (the controller has not observed the latest spec yet)
	// is not actually healthy; surface it as Unknown rather than counting it
	// toward the ready totals.
	if r.Ready == ReadyTrue && r.ObservedGeneration > 0 && r.Generation > r.ObservedGeneration {
		r.Ready = ReadyUnknown
	}
	r.Revision = extractRevision(obj)

	raw, err := json.Marshal(obj)
	if err != nil {
		return r, err
	}
	r.Raw = raw
	return r, nil
}

// applyReadyCondition fills ready/reason/message/lastTransition from the
// status condition of type "Ready" — the overall health signal every Flux
// kind publishes via meta.fluxcd.io.
func (r *Resource) applyReadyCondition(obj map[string]any) {
	conditions, ok, _ := unstructured.NestedSlice(obj, "status", "conditions")
	if !ok {
		return
	}
	for _, c := range conditions {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if condType, _, _ := unstructured.NestedString(cm, "type"); condType != "Ready" {
			continue
		}
		if status, ok, _ := unstructured.NestedString(cm, "status"); ok && status != "" {
			r.Ready = status
		}
		r.Reason, _, _ = unstructured.NestedString(cm, "reason")
		r.Message, _, _ = unstructured.NestedString(cm, "message")
		if lt, ok, _ := unstructured.NestedString(cm, "lastTransitionTime"); ok && lt != "" {
			if t, err := time.Parse(time.RFC3339, lt); err == nil {
				r.LastTransition = &t
			}
		}
		return
	}
}

// extractRevision returns a best-effort source/applied revision across the
// Flux kinds: Kustomization (status.lastAppliedRevision), sources
// (status.artifact.revision), HelmRelease (status.history[0].chartVersion),
// falling back to status.lastAttemptedRevision.
func extractRevision(obj map[string]any) string {
	if v, ok, _ := unstructured.NestedString(obj, "status", "lastAppliedRevision"); ok && v != "" {
		return v
	}
	if v, ok, _ := unstructured.NestedString(obj, "status", "artifact", "revision"); ok && v != "" {
		return v
	}
	if history, ok, _ := unstructured.NestedSlice(obj, "status", "history"); ok && len(history) > 0 {
		if h0, ok := history[0].(map[string]any); ok {
			if v, ok, _ := unstructured.NestedString(h0, "chartVersion"); ok && v != "" {
				return v
			}
		}
	}
	if v, ok, _ := unstructured.NestedString(obj, "status", "lastAttemptedRevision"); ok && v != "" {
		return v
	}
	return ""
}
