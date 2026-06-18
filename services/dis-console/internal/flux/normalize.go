package flux

import (
	"encoding/json"
	"fmt"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	// ContentHash is a stable digest of the full object (after volatile
	// metadata is stripped). The store rewrites a row's raw payload only when
	// this changes, so unchanged objects don't churn the database every sweep.
	// Internal bookkeeping, not part of the API payload.
	ContentHash string `json:"-"`
}

// normalize projects an unstructured Flux object into a Resource. The fetch
// stays dynamic (discovery-resolved, version-agnostic) and the full object is
// preserved for Raw; only the projected status fields are decoded into the
// typed Flux api structs — see the package doc and the dis-console README.
func normalize(u *unstructured.Unstructured) (Resource, error) {
	r := Resource{
		Kind:       u.GetKind(),
		APIVersion: u.GetAPIVersion(),
		Namespace:  u.GetNamespace(),
		Name:       u.GetName(),
		Generation: u.GetGeneration(),
		Ready:      ReadyUnknown,
	}

	if err := r.applyTypedStatus(u); err != nil {
		return r, err
	}
	// A stale Ready=True (the controller has not observed the latest spec yet)
	// is not actually healthy; surface it as Unknown rather than counting it
	// toward the ready totals.
	if r.Ready == ReadyTrue && r.ObservedGeneration > 0 && r.Generation > r.ObservedGeneration {
		r.Ready = ReadyUnknown
	}

	// Strip volatile metadata, hash, and (if oversized) truncate the stored
	// payload — see hygiene.go.
	raw, hash, err := rawAndHash(u.Object)
	if err != nil {
		return r, err
	}
	r.Raw = raw
	r.ContentHash = hash
	return r, nil
}

// applyTypedStatus decodes the object into its typed Flux struct and fills the
// projected status fields. The Ready condition is shared by every Flux kind
// (meta.fluxcd.io semantics); the revision source differs per kind.
func (r *Resource) applyTypedStatus(u *unstructured.Unstructured) error {
	switch u.GetKind() {
	case KindKustomization:
		var o kustomizev1.Kustomization
		if err := fromUnstructured(u, &o); err != nil {
			return err
		}
		revision := o.Status.LastAppliedRevision
		if revision == "" {
			revision = o.Status.LastAttemptedRevision
		}
		r.applyStatus(o.Spec.Suspend, o.Status.ObservedGeneration, o.Status.Conditions, revision)
	case KindHelmRelease:
		var o helmv2.HelmRelease
		if err := fromUnstructured(u, &o); err != nil {
			return err
		}
		var revision string
		if len(o.Status.History) > 0 {
			revision = o.Status.History[0].ChartVersion
		}
		if revision == "" {
			revision = o.Status.LastAttemptedRevision
		}
		r.applyStatus(o.Spec.Suspend, o.Status.ObservedGeneration, o.Status.Conditions, revision)
	case KindOCIRepository:
		var o sourcev1.OCIRepository
		if err := fromUnstructured(u, &o); err != nil {
			return err
		}
		r.applyStatus(o.Spec.Suspend, o.Status.ObservedGeneration, o.Status.Conditions, artifactRevision(o.Status.Artifact))
	case KindHelmRepository:
		var o sourcev1.HelmRepository
		if err := fromUnstructured(u, &o); err != nil {
			return err
		}
		r.applyStatus(o.Spec.Suspend, o.Status.ObservedGeneration, o.Status.Conditions, artifactRevision(o.Status.Artifact))
	case KindHelmChart:
		var o sourcev1.HelmChart
		if err := fromUnstructured(u, &o); err != nil {
			return err
		}
		r.applyStatus(o.Spec.Suspend, o.Status.ObservedGeneration, o.Status.Conditions, artifactRevision(o.Status.Artifact))
	}
	return nil
}

// applyStatus fills the projected fields from the typed values, taking the
// overall health from the Ready condition every Flux kind publishes.
func (r *Resource) applyStatus(suspended bool, observedGeneration int64, conditions []metav1.Condition, revision string) {
	r.Suspended = suspended
	r.ObservedGeneration = observedGeneration
	r.Revision = revision

	c := apimeta.FindStatusCondition(conditions, fluxmeta.ReadyCondition)
	if c == nil {
		return
	}
	if c.Status != "" {
		r.Ready = string(c.Status)
	}
	r.Reason = c.Reason
	r.Message = c.Message
	if !c.LastTransitionTime.Time.IsZero() {
		t := c.LastTransitionTime.Time
		r.LastTransition = &t
	}
}

// artifactRevision is the applied revision for the source kinds.
func artifactRevision(a *fluxmeta.Artifact) string {
	if a == nil {
		return ""
	}
	return a.Revision
}

// fromUnstructured decodes the (version-agnostically fetched) object into a
// typed Flux struct. It maps by JSON field, ignoring unknown/extra fields, so
// it stays tolerant of whichever served version the cluster exposes.
func fromUnstructured(u *unstructured.Unstructured, into any) error {
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, into); err != nil {
		return fmt.Errorf("decode %s %s/%s: %w", u.GetKind(), u.GetNamespace(), u.GetName(), err)
	}
	return nil
}
