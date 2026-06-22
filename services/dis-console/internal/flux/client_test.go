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
// List and returns an empty list. We capture the options here rather than via
// dynamic/fake because the fake client drops ResourceVersion when building its
// recorded action, which is exactly the field these tests assert on.
type fakeDynamic struct {
	dynamic.Interface
	listOpts []metav1.ListOptions
}

func (d *fakeDynamic) Resource(schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &fakeResource{parent: d}
}

type fakeResource struct {
	dynamic.NamespaceableResourceInterface
	parent *fakeDynamic
}

func (r *fakeResource) Namespace(string) dynamic.ResourceInterface { return r }

func (r *fakeResource) List(_ context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	r.parent.listOpts = append(r.parent.listOpts, opts)
	return &unstructured.UnstructuredList{}, nil
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
