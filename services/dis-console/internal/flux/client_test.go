package flux

import (
	"context"
	"strings"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
// List and returns the items seeded for the resource (empty when none). We
// capture the options here rather than via dynamic/fake because the fake
// client drops ResourceVersion when building its recorded action, which is
// exactly the field these tests assert on.
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
	return &unstructured.UnstructuredList{Items: r.parent.items[r.resource]}, nil
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
	if len(d.listOpts) != len(TargetKinds) {
		t.Fatalf("list calls = %d, want %d (one per target kind)", len(d.listOpts), len(TargetKinds))
	}
	for i, opts := range d.listOpts {
		if opts.ResourceVersion != "0" {
			t.Errorf("list[%d] ResourceVersion = %q, want %q (served from the watch cache)",
				i, opts.ResourceVersion, "0")
		}
		// The apps kinds are filtered to GitOps-applied objects server-side;
		// every other kind lists unfiltered.
		wantSelector := ""
		if TargetKinds[i].Group == GroupApps {
			wantSelector = LabelAppliedByName
		}
		if opts.LabelSelector != wantSelector {
			t.Errorf("list[%d] LabelSelector = %q, want %q", i, opts.LabelSelector, wantSelector)
		}
	}
}

// TestSweepMirrorsOnlyGitOpsAppliedWorkloads pins the workload filter: an apps
// object without the kustomize-controller ownership label (kube-system,
// Azure-managed add-ons) is dropped before normalize, while the labeled one is
// mirrored — and non-workload kinds are never filtered, labeled or not.
func TestSweepMirrorsOnlyGitOpsAppliedWorkloads(t *testing.T) {
	workload := func(name string, labels map[string]any) unstructured.Unstructured {
		meta := map[string]any{"namespace": "ns", "name": name}
		if labels != nil {
			meta["labels"] = labels
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
	if len(d.listOpts) != len(TargetKinds)-1 {
		t.Fatalf("list calls = %d, want %d (skipped kind is not listed)", len(d.listOpts), len(TargetKinds)-1)
	}
}
