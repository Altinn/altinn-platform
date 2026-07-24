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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	Parent *ParentRef `json:"parent,omitempty"`
	// AppliedBy is the Kustomization that applied this object, projected from
	// the kustomize.toolkit.fluxcd.io/{name,namespace} labels the kustomize
	// controller stamps on everything it applies — or, for chart-created
	// workloads (which carry no kustomize labels), the owning HelmRelease,
	// resolved by Sweep from the object's Helm release annotations. Lets the
	// list endpoint (which omits Raw) group child resources under their parent
	// app. Empty for roots, Arc-managed objects, and Helm releases installed
	// outside Flux.
	AppliedBy *AppliedBy `json:"appliedBy,omitempty"`
	// SourceRef is the Flux source a Kustomization builds from — the join key
	// from a Kustomization row to the OCIRepository row holding the base-layer
	// artifact it deploys. Nil for every other kind.
	SourceRef *SourceRef `json:"sourceRef,omitempty"`
	// SourceURL is a source kind's artifact URL (OCIRepository/HelmRepository
	// spec.url). It is the only identity a base-layer artifact carries — the
	// CRs have no product/team labels — so the artifacts view classifies on it.
	SourceURL string `json:"sourceUrl,omitempty"`
	// OriginRevision/OriginSource are the artifact's org.opencontainers.image
	// revision/source annotations (stamped by `flux push artifact --revision
	// --source`), surfaced by source-controller in status.artifact.metadata:
	// the git branch/SHA and repository behind the artifact digest. Empty when
	// the pusher did not annotate.
	OriginRevision string `json:"originRevision,omitempty"`
	OriginSource   string `json:"originSource,omitempty"`
	// Images are a workload's container images from its pod template
	// (spec.template.spec.containers; init containers are skipped) — the
	// app's effective version, which the manifest revision cannot show when
	// the tag is resolved per cluster via postBuild substitution. Rides in
	// list payloads (unlike Raw/Inventory) so the UI needs no detail fetch.
	// Nil for every non-workload kind.
	Images             []ContainerImage `json:"images,omitempty"`
	Suspended          bool             `json:"suspended"`
	Generation         int64            `json:"generation,omitempty"`
	ObservedGeneration int64            `json:"observedGeneration,omitempty"`
	LastTransition     *time.Time       `json:"lastTransition,omitempty"`
	// Inventory is a Kustomization's applied-object set (status.inventory) —
	// the parent→children edge of the deployment tree, covering kinds the agent
	// does not sweep. Like Raw it is served on detail endpoints only; list
	// payloads omit it. Kept in Flux's compact entry shape (the source object
	// is bounded by etcd, so the projection needs no cap of its own).
	Inventory []InventoryEntry `json:"inventory,omitempty"`
	Raw       json.RawMessage  `json:"raw,omitempty"`
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

// Labels the kustomize controller stamps on every object it applies, naming
// the owning Kustomization. HelmReleases applied by a Kustomization carry
// them too, which is how the UI groups child HelmReleases under their app.
const (
	LabelAppliedByName      = "kustomize.toolkit.fluxcd.io/name"
	LabelAppliedByNamespace = "kustomize.toolkit.fluxcd.io/namespace"
)

// Metadata Helm stamps on every object it renders: the managed-by label the
// sweep's second workload list selects on, and the release annotations naming
// the owning Helm release. Chart objects carry these instead of the kustomize
// labels — helm-controller applies them itself, not via kustomize-controller.
const (
	labelManagedBy             = "app.kubernetes.io/managed-by"
	managedByHelm              = "Helm"
	annotationReleaseName      = "meta.helm.sh/release-name"
	annotationReleaseNamespace = "meta.helm.sh/release-namespace"
)

// AppliedBy identifies the Kustomization that applied a resource. The JSON
// shape (name/namespace) is part of the UI contract.
type AppliedBy struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// SourceRef identifies the Flux source a Kustomization builds from
// (spec.sourceRef). Namespace is resolved to the Kustomization's own namespace
// when the reference omits it, so consumers can join without knowing the
// defaulting rule. The JSON shape is part of the UI contract.
type SourceRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// ContainerImage is one container of a workload's pod template and the image
// it runs. The JSON shape (container/image) is part of the UI contract.
type ContainerImage struct {
	Container string `json:"container"`
	Image     string `json:"image"`
}

// InventoryEntry is one applied-object reference from a Kustomization's
// status.inventory, in Flux's compact wire shape: ID is
// `<namespace>_<name>_<group>_<kind>` (namespace empty for cluster-scoped
// objects) and Version the object's API version. The API expands it when
// serving; the store keeps the compact form.
type InventoryEntry struct {
	ID      string `json:"id"`
	Version string `json:"v"`
}

// Annotation keys `flux push artifact` stamps on an artifact and
// source-controller copies into status.artifact.metadata.
const (
	annotationOriginRevision = "org.opencontainers.image.revision"
	annotationOriginSource   = "org.opencontainers.image.source"
)

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
		AppliedBy:  appliedByFrom(u.GetLabels()),
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
// and the apps workloads take their own projection paths (applyDISStatus,
// applyWorkloadStatus).
func (r *Resource) applyTypedStatus(u *unstructured.Unstructured) error {
	switch group := u.GroupVersionKind().Group; {
	case isDISGroup(group):
		return r.applyDISStatus(u)
	case group == GroupApps:
		return r.applyWorkloadStatus(u)
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
		r.SourceRef = kustomizationSourceRef(&o)
		r.Inventory = inventoryEntries(o.Status.Inventory)
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
		r.SourceURL = o.Spec.URL
		r.OriginRevision, r.OriginSource = artifactOrigin(o.Status.Artifact)
	case KindHelmRepository:
		var o sourcev1.HelmRepository
		if err := fromUnstructured(u, &o); err != nil {
			return err
		}
		r.applyStatus(o.Spec.Suspend, o.Status.ObservedGeneration, o.Status.Conditions, artifactRevision(o.Status.Artifact))
		r.SourceURL = o.Spec.URL
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

// artifactOrigin extracts the artifact's origin annotations — the git
// branch/SHA and repository the artifact was pushed from. Empty when the
// pusher did not annotate (only `flux push artifact --revision --source`
// stamps them).
func artifactOrigin(a *fluxmeta.Artifact) (revision, source string) {
	if a == nil {
		return "", ""
	}
	return a.Metadata[annotationOriginRevision], a.Metadata[annotationOriginSource]
}

// kustomizationSourceRef projects spec.sourceRef with the namespace default
// resolved (an omitted namespace means the Kustomization's own).
func kustomizationSourceRef(o *kustomizev1.Kustomization) *SourceRef {
	ref := o.Spec.SourceRef
	if ref.Name == "" {
		return nil
	}
	ns := ref.Namespace
	if ns == "" {
		ns = o.Namespace
	}
	return &SourceRef{Kind: ref.Kind, Name: ref.Name, Namespace: ns}
}

// inventoryEntries projects status.inventory into the stored entry shape; nil
// when the Kustomization has not recorded an inventory (never reconciled).
func inventoryEntries(inv *kustomizev1.ResourceInventory) []InventoryEntry {
	if inv == nil || len(inv.Entries) == 0 {
		return nil
	}
	out := make([]InventoryEntry, len(inv.Entries))
	for i, e := range inv.Entries {
		out[i] = InventoryEntry{ID: e.ID, Version: e.Version}
	}
	return out
}

// appliedByFrom projects the kustomize-controller ownership labels into an
// AppliedBy. Returns nil when the labels are absent (roots, Arc-managed
// objects) so the JSON field is omitted.
func appliedByFrom(labels map[string]string) *AppliedBy {
	name, ns := labels[LabelAppliedByName], labels[LabelAppliedByNamespace]
	if name == "" && ns == "" {
		return nil
	}
	return &AppliedBy{Name: name, Namespace: ns}
}

// helmReleaseIdentity is the effective Helm release identity a HelmRelease
// deploys as — the exact values Helm stamps into the meta.helm.sh release
// annotations of every object it renders. The typed helpers encode Flux's
// defaulting: the name is spec.releaseName when set, else
// <targetNamespace>-<name> when spec.targetNamespace is set, else the CR's
// name; the namespace is spec.targetNamespace defaulting to the CR's own.
// (spec.storageNamespace moves only the release secrets: helm-controller
// hands Helm the release namespace, not the storage namespace, so the latter
// never reaches object metadata.)
func helmReleaseIdentity(u *unstructured.Unstructured) (types.NamespacedName, error) {
	var o helmv2.HelmRelease
	if err := fromUnstructured(u, &o); err != nil {
		return types.NamespacedName{}, err
	}
	return types.NamespacedName{Namespace: o.GetReleaseNamespace(), Name: o.GetReleaseName()}, nil
}

// helmOwnerFrom reads the meta.helm.sh release annotations naming the Helm
// release an object belongs to. ok is false when they are absent — an object
// that merely carries a chart-hardcoded managed-by label but was applied by
// something other than Helm.
func helmOwnerFrom(annotations map[string]string) (types.NamespacedName, bool) {
	name, ns := annotations[annotationReleaseName], annotations[annotationReleaseNamespace]
	if name == "" || ns == "" {
		return types.NamespacedName{}, false
	}
	return types.NamespacedName{Namespace: ns, Name: name}, true
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

// applyWorkloadStatus fills the projected fields for an apps workload, decoded
// into the typed k8s.io/api structs the same runtime-conversion way as the
// Flux kinds. All three kinds project their pod template's container images;
// readiness is per kind because they share no condition semantics: Deployment
// publishes an Available condition, StatefulSet and DaemonSet publish only
// replica counts, which are synthesized into a short reason/message.
func (r *Resource) applyWorkloadStatus(u *unstructured.Unstructured) error {
	switch u.GetKind() {
	case KindDeployment:
		var o appsv1.Deployment
		if err := fromUnstructured(u, &o); err != nil {
			return err
		}
		// paused is the workload analogue of a suspended Flux object:
		// intentionally not being reconciled.
		r.Suspended = o.Spec.Paused
		r.ObservedGeneration = o.Status.ObservedGeneration
		r.Images = containerImages(o.Spec.Template.Spec.Containers)
		for _, c := range o.Status.Conditions {
			if c.Type != appsv1.DeploymentAvailable {
				continue
			}
			r.Ready = string(c.Status)
			r.Reason = c.Reason
			r.Message = c.Message
			if !c.LastTransitionTime.Time.IsZero() {
				t := c.LastTransitionTime.Time
				r.LastTransition = &t
			}
		}
	case KindStatefulSet:
		var o appsv1.StatefulSet
		if err := fromUnstructured(u, &o); err != nil {
			return err
		}
		r.ObservedGeneration = o.Status.ObservedGeneration
		r.Images = containerImages(o.Spec.Template.Spec.Containers)
		desired := int32(1) // nil spec.replicas defaults to 1
		if o.Spec.Replicas != nil {
			desired = *o.Spec.Replicas
		}
		r.applyReadyReplicas(o.Status.ReadyReplicas, desired)
	case KindDaemonSet:
		var o appsv1.DaemonSet
		if err := fromUnstructured(u, &o); err != nil {
			return err
		}
		r.ObservedGeneration = o.Status.ObservedGeneration
		r.Images = containerImages(o.Spec.Template.Spec.Containers)
		r.applyReadyReplicas(o.Status.NumberReady, o.Status.DesiredNumberScheduled)
	}
	return nil
}

// applyReadyReplicas derives readiness from a workload's replica counts (the
// kinds without a usable condition): every desired replica ready is healthy —
// including a scaled-to-zero 0/0. An object its controller has never observed
// (observedGeneration 0) stays Unknown rather than claiming 0/0 ready; a stale
// True is downgraded by the generic observedGeneration check in normalize.
func (r *Resource) applyReadyReplicas(ready, desired int32) {
	if r.ObservedGeneration == 0 {
		return
	}
	r.Ready = ReadyFalse
	if ready == desired {
		r.Ready = ReadyTrue
	}
	r.Reason = "ReadyReplicas"
	r.Message = fmt.Sprintf("%d/%d ready", ready, desired)
}

// containerImages projects a pod template's containers into the images list;
// nil when there are none. Init containers are deliberately skipped: the
// long-running containers are what carries the app's version.
func containerImages(containers []corev1.Container) []ContainerImage {
	if len(containers) == 0 {
		return nil
	}
	out := make([]ContainerImage, len(containers))
	for i, c := range containers {
		out[i] = ContainerImage{Container: c.Name, Image: c.Image}
	}
	return out
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
