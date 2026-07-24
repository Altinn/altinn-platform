package flux

import (
	"context"
	"strings"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// fakeMapper is a ResettableRESTMapper that counts Reset calls and resolves every
// GroupKind to a synthetic v1 mapping, except those marked in noMatch, which
// return a NoKindMatchError so IsNoMatchError treats them as "not installed".
// The embedded interface is nil: only RESTMapping (overridden) and Reset are
// exercised by Sweep.
type fakeMapper struct {
	apimeta.RESTMapper
	resetCount int
	noMatch    map[schema.GroupKind]bool
}

func (m *fakeMapper) Reset() { m.resetCount++ }

func (m *fakeMapper) RESTMapping(gk schema.GroupKind, _ ...string) (*apimeta.RESTMapping, error) {
	if m.noMatch[gk] {
		return nil, &apimeta.NoKindMatchError{GroupKind: gk}
	}
	gvk := gk.WithVersion("v1")
	return &apimeta.RESTMapping{
		Resource:         gvk.GroupVersion().WithResource(strings.ToLower(gk.Kind) + "s"),
		GroupVersionKind: gvk,
		Scope:            apimeta.RESTScopeNamespace,
	}, nil
}

// fakeDynamic is a dynamic.Interface that records the ListOptions of every
// List and returns the items seeded for the resource (empty when none),
// filtered by the request's label selector like the real apiserver — Sweep
// leans on server-side selection for the workload kinds, so an unfiltering
// fake would leak items into the wrong pass. We capture the options here
// rather than via dynamic/fake because the fake client drops ResourceVersion
// when building its recorded action, which is exactly the field these tests
// assert on.
type fakeDynamic struct {
	dynamic.Interface
	listOpts []metav1.ListOptions
	// items are the objects List returns, keyed by resource (e.g. "deployments").
	items map[string][]unstructured.Unstructured
}

func (d *fakeDynamic) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &fakeResource{parent: d, resource: gvr.Resource}
}

type fakeResource struct {
	dynamic.NamespaceableResourceInterface
	parent   *fakeDynamic
	resource string
}

func (r *fakeResource) Namespace(string) dynamic.ResourceInterface { return r }

func (r *fakeResource) List(_ context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	r.parent.listOpts = append(r.parent.listOpts, opts)
	selector := labels.Everything()
	if opts.LabelSelector != "" {
		var err error
		if selector, err = labels.Parse(opts.LabelSelector); err != nil {
			return nil, err
		}
	}
	out := &unstructured.UnstructuredList{}
	for _, item := range r.parent.items[r.resource] {
		if selector.Matches(labels.Set(item.GetLabels())) {
			out.Items = append(out.Items, item)
		}
	}
	return out, nil
}

func TestSweepResetsDiscoveryOnTTL(t *testing.T) {
	m := &fakeMapper{}
	c := &Client{dyn: &fakeDynamic{}, mapper: m}
	ctx := context.Background()

	// The first sweep always resets: the zero-value lastDiscovery is far past the
	// TTL (time.Since saturates), so discovery is fetched once at startup.
	if _, _, err := c.Sweep(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if m.resetCount != 1 {
		t.Fatalf("first sweep reset count = %d, want 1", m.resetCount)
	}

	// A second sweep within the TTL must not reset discovery again.
	if _, _, err := c.Sweep(ctx); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if m.resetCount != 1 {
		t.Fatalf("reset count after sweep within TTL = %d, want 1", m.resetCount)
	}

	// Once the TTL has elapsed, the next sweep resets discovery again.
	c.lastDiscovery = time.Now().Add(-discoveryTTL - time.Minute)
	if _, _, err := c.Sweep(ctx); err != nil {
		t.Fatalf("third sweep: %v", err)
	}
	if m.resetCount != 2 {
		t.Fatalf("reset count after TTL elapsed = %d, want 2", m.resetCount)
	}
}

func TestSweepListsFromWatchCache(t *testing.T) {
	d := &fakeDynamic{}
	c := &Client{dyn: d, mapper: &fakeMapper{}}

	if _, _, err := c.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	// The apps kinds are filtered server-side and listed twice (the
	// kustomize-applied and the Helm-rendered populations); every other kind
	// lists once, unfiltered.
	var wantSelectors []string
	for _, gk := range TargetKinds {
		if gk.Group == GroupApps {
			wantSelectors = append(wantSelectors, LabelAppliedByName, helmManagedSelector)
			continue
		}
		wantSelectors = append(wantSelectors, "")
	}
	if len(d.listOpts) != len(wantSelectors) {
		t.Fatalf("list calls = %d, want %d (one per target kind, two per apps kind)",
			len(d.listOpts), len(wantSelectors))
	}
	for i, opts := range d.listOpts {
		if opts.ResourceVersion != "0" {
			t.Errorf("list[%d] ResourceVersion = %q, want %q (served from the watch cache)",
				i, opts.ResourceVersion, "0")
		}
		if opts.LabelSelector != wantSelectors[i] {
			t.Errorf("list[%d] LabelSelector = %q, want %q", i, opts.LabelSelector, wantSelectors[i])
		}
	}
}

// TestSweepMirrorsOnlyGitOpsAppliedWorkloads pins the workload filter: an apps
// object without the kustomize-controller ownership label (kube-system,
// Azure-managed add-ons) is dropped before normalize, while the labeled one is
// mirrored — and non-workload kinds are never filtered, labeled or not.
func TestSweepMirrorsOnlyGitOpsAppliedWorkloads(t *testing.T) {
	workload := func(name string, objLabels map[string]any) unstructured.Unstructured {
		meta := map[string]any{"namespace": "ns", "name": name}
		if objLabels != nil {
			meta["labels"] = objLabels
		}
		return unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   meta,
		}}
	}
	d := &fakeDynamic{items: map[string][]unstructured.Unstructured{
		"deployments": {
			workload("gitops-app", map[string]any{LabelAppliedByName: "app", LabelAppliedByNamespace: "ns"}),
			workload("azure-managed", nil),
		},
		// An unlabeled non-workload kind must not be filtered.
		"kustomizations": {{Object: map[string]any{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata":   map[string]any{"namespace": "flux-system", "name": "root"},
		}}},
	}}
	c := &Client{dyn: d, mapper: &fakeMapper{}}

	resources, warnings, err := c.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	names := make([]string, len(resources))
	for i, r := range resources {
		names[i] = r.Name
	}
	if len(resources) != 2 || resources[0].Name != "root" || resources[1].Name != "gitops-app" {
		t.Fatalf("expected the root Kustomization and the labeled workload only, got %v", names)
	}
	if resources[1].AppliedBy == nil || resources[1].AppliedBy.Name != "app" {
		t.Fatalf("mirrored workload lost appliedBy: %+v", resources[1].AppliedBy)
	}
}

// TestSweepResolvesHelmWorkloadOwnership pins the Helm side of the workload
// sweep: chart-created workloads (managed-by label + release annotations, no
// kustomize labels) are mirrored with appliedBy resolved to the HelmRelease
// deploying that release — through the default identity, a releaseName
// override, and targetNamespace prefix defaulting. A kustomize-applied object
// whose chart hardcodes the managed-by label is mirrored exactly once and
// keeps the kustomize appliedBy; a release no HelmRelease accounts for stays
// mirrored without an owner.
func TestSweepResolvesHelmWorkloadOwnership(t *testing.T) {
	deployment := func(ns, name string, objLabels, annotations map[string]any) unstructured.Unstructured {
		meta := map[string]any{"namespace": ns, "name": name}
		if objLabels != nil {
			meta["labels"] = objLabels
		}
		if annotations != nil {
			meta["annotations"] = annotations
		}
		return unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   meta,
		}}
	}
	helmRelease := func(ns, name string, spec map[string]any) unstructured.Unstructured {
		obj := map[string]any{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata":   map[string]any{"namespace": ns, "name": name},
		}
		if spec != nil {
			obj["spec"] = spec
		}
		return unstructured.Unstructured{Object: obj}
	}
	helmLabel := map[string]any{labelManagedBy: managedByHelm}

	d := &fakeDynamic{items: map[string][]unstructured.Unstructured{
		"helmreleases": {
			helmRelease("team-one", "app-default", nil),
			helmRelease("team-two", "app-target", map[string]any{"targetNamespace": "team-two-apps"}),
			helmRelease("team-three", "app-renamed", map[string]any{"releaseName": "custom-release"}),
		},
		"deployments": {
			// Default identity: release name = CR name, namespace = CR's own.
			deployment("team-one", "worker-default", helmLabel, map[string]any{
				annotationReleaseName:      "app-default",
				annotationReleaseNamespace: "team-one",
			}),
			// targetNamespace defaulting: the release is named
			// <targetNamespace>-<name> and lives in the target namespace, but
			// appliedBy must name the CR (app-target in team-two).
			deployment("team-two-apps", "worker-target", helmLabel, map[string]any{
				annotationReleaseName:      "team-two-apps-app-target",
				annotationReleaseNamespace: "team-two-apps",
			}),
			// spec.releaseName overrides the release name outright.
			deployment("team-three", "worker-renamed", helmLabel, map[string]any{
				annotationReleaseName:      "custom-release",
				annotationReleaseNamespace: "team-three",
			}),
			// Matches both selectors (chart-hardcoded managed-by label on a
			// kustomize-applied object): mirrored once, kustomize wins.
			deployment("product-team-a", "both", map[string]any{
				LabelAppliedByName:      "team-a-app",
				LabelAppliedByNamespace: "product-team-a",
				labelManagedBy:          managedByHelm,
			}, nil),
			// A release no HelmRelease accounts for (helm install by hand).
			deployment("tools", "hand-rolled", helmLabel, map[string]any{
				annotationReleaseName:      "toolbox",
				annotationReleaseNamespace: "tools",
			}),
			// Hardcoded label, no release annotations: nothing to resolve.
			deployment("misc", "label-only", helmLabel, nil),
		},
	}}
	c := &Client{dyn: d, mapper: &fakeMapper{}}

	resources, warnings, err := c.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	byName := make(map[string]Resource, len(resources))
	for _, r := range resources {
		if prev, dup := byName[r.Name]; dup {
			t.Fatalf("resource %q mirrored twice: %+v and %+v", r.Name, prev, r)
		}
		byName[r.Name] = r
	}
	if len(resources) != 9 {
		t.Fatalf("resources = %d, want 9 (3 HelmReleases + 6 deployments, each once)", len(resources))
	}

	// Every appliedBy names the HelmRelease CR, never the helm release —
	// worker-renamed's release is custom-release but its owner is the
	// app-renamed CR.
	wantOwner := map[string]*AppliedBy{
		"worker-default": {Name: "app-default", Namespace: "team-one"},
		"worker-target":  {Name: "app-target", Namespace: "team-two"},
		"worker-renamed": {Name: "app-renamed", Namespace: "team-three"},
		"both":           {Name: "team-a-app", Namespace: "product-team-a"},
		"hand-rolled":    nil,
		"label-only":     nil,
	}
	for name, want := range wantOwner {
		got, ok := byName[name]
		if !ok {
			t.Fatalf("workload %q not mirrored", name)
		}
		switch {
		case want == nil:
			if got.AppliedBy != nil {
				t.Errorf("%s appliedBy = %+v, want none", name, got.AppliedBy)
			}
		case got.AppliedBy == nil:
			t.Errorf("%s appliedBy = nil, want %+v", name, want)
		case *got.AppliedBy != *want:
			t.Errorf("%s appliedBy = %+v, want %+v", name, got.AppliedBy, want)
		}
	}
}

func TestSweepSkipsUninstalledKinds(t *testing.T) {
	// Mark one optional source kind as not installed.
	missing := schema.GroupKind{Group: GroupSource, Kind: KindHelmChart}
	d := &fakeDynamic{}
	c := &Client{dyn: d, mapper: &fakeMapper{noMatch: map[schema.GroupKind]bool{missing: true}}}

	_, warnings, err := c.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep aborted on an uninstalled kind, want skip: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %d, want 1 (the skipped kind)", len(warnings))
	}
	// One list per kind minus the skipped one, plus a second list for each of
	// the three apps kinds.
	want := len(TargetKinds) - 1 + 3
	if len(d.listOpts) != want {
		t.Fatalf("list calls = %d, want %d (skipped kind is not listed)", len(d.listOpts), want)
	}
}
