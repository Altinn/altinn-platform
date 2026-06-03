// Package flux reads Flux custom resources across all namespaces using the
// dynamic client and a discovery-backed RESTMapper, and normalizes their
// deployment status into a small, stable shape the API serves.
package flux

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// Flux API groups and the kinds we key on by name.
const (
	GroupKustomize = "kustomize.toolkit.fluxcd.io"
	GroupHelm      = "helm.toolkit.fluxcd.io"
	GroupSource    = "source.toolkit.fluxcd.io"

	KindKustomization = "Kustomization"
	KindHelmRelease   = "HelmRelease"
)

// TargetKinds are the Flux custom resource kinds the Console reads. The served
// API version of each is resolved at runtime via the discovery RESTMapper, so
// the same binary keeps working if Azure Flux bumps a version.
var TargetKinds = []schema.GroupKind{
	{Group: GroupKustomize, Kind: KindKustomization},
	{Group: GroupHelm, Kind: KindHelmRelease},
	{Group: GroupSource, Kind: "OCIRepository"},
	{Group: GroupSource, Kind: "HelmRepository"},
	{Group: GroupSource, Kind: "HelmChart"},
}

// Client lists Flux custom resources across all namespaces.
type Client struct {
	dyn    dynamic.Interface
	mapper *restmapper.DeferredDiscoveryRESTMapper
}

// NewClient builds a Flux client. When local is true it uses the default
// kubeconfig (laptop dev); otherwise it uses the in-cluster config.
func NewClient(local bool) (*Client, error) {
	cfg, err := restConfig(local)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("discovery client: %w", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disco))
	return &Client{dyn: dyn, mapper: mapper}, nil
}

func restConfig(local bool) (*rest.Config, error) {
	if local {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		overrides := &clientcmd.ConfigOverrides{}
		loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
		cfg, err := loader.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("kubeconfig: %w", err)
		}
		return cfg, nil
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}
	return cfg, nil
}

// Sweep lists every instance of each target kind across all namespaces and
// returns them as normalized resources. Kinds that are not installed are
// skipped (reported as warnings) so the sweep still succeeds when only a
// subset of Flux controllers is present. Any other discovery/list failure
// (RBAC, auth, apiserver outage) aborts the sweep with an error so the caller
// keeps the previous snapshot instead of publishing a partial one. The
// discovery cache is refreshed each sweep so newly installed CRDs are picked up.
func (c *Client) Sweep(ctx context.Context) ([]Resource, []error, error) {
	c.mapper.Reset()

	resources := make([]Resource, 0)
	var warnings []error

	for _, gk := range TargetKinds {
		mapping, err := c.mapper.RESTMapping(gk)
		if err != nil {
			// A kind that isn't installed is expected (optional source kinds);
			// skip it. Any other mapping error is a discovery/access failure and
			// must abort the sweep.
			if apimeta.IsNoMatchError(err) {
				warnings = append(warnings, fmt.Errorf("skip %s: not installed", gk))
				continue
			}
			return nil, warnings, fmt.Errorf("resolve %s: %w", gk, err)
		}
		nri := c.dyn.Resource(mapping.Resource).Namespace(metav1.NamespaceAll)
		list, err := nri.List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, warnings, fmt.Errorf("list %s: %w", gk, err)
		}
		for i := range list.Items {
			r, err := normalize(&list.Items[i])
			if err != nil {
				warnings = append(warnings, fmt.Errorf("normalize %s: %w", gk, err))
				continue
			}
			resources = append(resources, r)
		}
	}
	return resources, warnings, nil
}
