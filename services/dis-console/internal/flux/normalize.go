package flux

import (
	"encoding/json"
	"fmt"
	"strings"
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

// Resource is a normalized view of a Flux or DIS custom resource's deployment
// status. Only the fields needed for the summary and "what's broken" views are
// pulled out; the full object is preserved verbatim in Raw so the detail
// endpoint can expose anything else without a schema change.
type Resource struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	Ready      string `json:"ready"` // True | False | Unknown
	Reason     string `json:"reason,omitempty"`
	Message    string `json:"message,omitempty"`
	Revision   string `json:"revision,omitempty"`
	// AzureResourceID is the ARM id of the Azure resource a DIS object
	// provisions (the UI builds Portal links from it). Empty for Flux kinds and
	// for DIS kinds whose operator has not published it yet.
	AzureResourceID string `json:"azureResourceId,omitempty"`
	// Parent names the same-namespace resource this one nests under in the UI
	// (a Database under its DatabaseServer, an ApiVersion under its Api).
	Parent             *ParentRef      `json:"parent,omitempty"`
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

// ParentRef identifies the resource another resource nests under. The JSON
// shape (kind/name) is part of the UI contract.
type ParentRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
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
// (meta.fluxcd.io semantics); the revision source differs per kind. DIS kinds
// take their own projection path (see applyDISStatus).
func (r *Resource) applyTypedStatus(u *unstructured.Unstructured) error {
	if isDISGroup(u.GroupVersionKind().Group) {
		return r.applyDISStatus(u)
	}
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

// disObject is the minimal projection of a DIS custom resource — just the
// fields the console reads, decoded the same runtime-conversion way as the
// Flux kinds. A local struct instead of the operator api packages keeps the
// operator modules (and the ASO dependency tree they drag in) out of this
// service. Fields a kind does not publish simply decode to their zero value.
type disObject struct {
	Spec struct {
		// Server names the same-namespace DatabaseServer a Database runs on.
		Server struct {
			Name string `json:"name"`
		} `json:"server"`
	} `json:"spec"`
	Status struct {
		// ResourceID is the ARM id of the provisioned Azure resource. Vault
		// publishes it today; DatabaseServer and ApplicationIdentity are
		// expected to adopt the same field.
		ResourceID string `json:"resourceId"`
		// APIVersionSetID and BackendID are the ARM ids the APIM kinds publish.
		APIVersionSetID string `json:"apiVersionSetID"`
		BackendID       string `json:"backendID"`
		// ProvisioningState is the only health signal the APIM kinds publish
		// (they set no conditions): Succeeded, Failed, Updating, Deleting or
		// Deleted.
		ProvisioningState  string             `json:"provisioningState"`
		ObservedGeneration int64              `json:"observedGeneration"`
		Conditions         []metav1.Condition `json:"conditions"`
	} `json:"status"`
}

// applyDISStatus fills the projected fields for a DIS custom resource. The
// storage/vault/application operators publish a Ready condition with the same
// semantics as Flux; the APIM kinds publish only status.provisioningState, so
// health is mapped from that when no Ready condition exists.
func (r *Resource) applyDISStatus(u *unstructured.Unstructured) error {
	var o disObject
	if err := fromUnstructured(u, &o); err != nil {
		return err
	}
	st := o.Status

	r.applyStatus(false, st.ObservedGeneration, st.Conditions, "")
	if apimeta.FindStatusCondition(st.Conditions, fluxmeta.ReadyCondition) == nil && st.ProvisioningState != "" {
		r.Ready = readyFromProvisioningState(st.ProvisioningState)
		r.Reason = st.ProvisioningState
	}

	// The ARM id behind the Portal link, from whichever status field the
	// kind's operator publishes. Database and ApiVersion have none.
	switch u.GetKind() {
	case KindVault, KindDatabaseServer, KindApplicationIdentity:
		r.AzureResourceID = st.ResourceID
	case KindApi:
		r.AzureResourceID = st.APIVersionSetID
	case KindBackend:
		r.AzureResourceID = st.BackendID
	}

	switch u.GetKind() {
	case KindDatabase:
		if name := o.Spec.Server.Name; name != "" {
			r.Parent = &ParentRef{Kind: KindDatabaseServer, Name: name}
		}
	case KindApiVersion:
		// The Api controller creates ApiVersions with a controller owner
		// reference; a user-created ApiVersion has none and stays top-level.
		for _, ref := range u.GetOwnerReferences() {
			if ref.Kind == KindApi && strings.HasPrefix(ref.APIVersion, GroupDISApim+"/") {
				r.Parent = &ParentRef{Kind: KindApi, Name: ref.Name}
				break
			}
		}
	}
	return nil
}

// readyFromProvisioningState maps the APIM operator's provisioningState onto
// the console's ready values: Succeeded is healthy, Failed is broken, and the
// transitional states (Updating, Deleting, Deleted) stay Unknown.
func readyFromProvisioningState(state string) string {
	switch state {
	case "Succeeded":
		return ReadyTrue
	case "Failed":
		return ReadyFalse
	default:
		return ReadyUnknown
	}
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
