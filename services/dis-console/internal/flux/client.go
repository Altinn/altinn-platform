// Package flux reads Flux and DIS custom resources across all namespaces using
// the dynamic client and a discovery-backed RESTMapper, and normalizes their
// deployment status into a small, stable shape the API serves.
//
// The fetch stays dynamic on purpose (the discovery RESTMapper resolves
// whichever served version Azure Flux exposes, and the full object is kept
// verbatim for the raw payload); the projected status fields are then decoded
// into the typed Flux api structs via runtime conversion — typed access without
// a version-pinned typed client. The DIS kinds are decoded the same way into
// minimal local structs so the operator modules stay out of the dependency
// graph. See normalize.go and the README.
package flux

import (
	"context"
	"fmt"
	"time"

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

	KindKustomization  = "Kustomization"
	KindHelmRelease    = "HelmRelease"
	KindOCIRepository  = "OCIRepository"
	KindHelmRepository = "HelmRepository"
	KindHelmChart      = "HelmChart"
)

// DIS platform API groups (one per operator) and their kinds. The kind names
// mirror the CRD spellings (Api, ApiVersion), not Go initialism style.
const (
	GroupDISStorage     = "storage.dis.altinn.cloud"
	GroupDISVault       = "vault.dis.altinn.cloud"
	GroupDISApplication = "application.dis.altinn.cloud"
	GroupDISApim        = "apim.dis.altinn.cloud"

	KindDatabaseServer      = "DatabaseServer"
	KindDatabase            = "Database"
	KindVault               = "Vault"
	KindApplicationIdentity = "ApplicationIdentity"
	KindApi                 = "Api"
	KindApiVersion          = "ApiVersion"
	KindBackend             = "Backend"
)

// isDISGroup reports whether group is one of the DIS platform API groups, which
// normalize projects via the DIS status shape instead of the typed Flux structs.
func isDISGroup(group string) bool {
	switch group {
	case GroupDISStorage, GroupDISVault, GroupDISApplication, GroupDISApim:
		return true
	}
	return false
}

// TargetKinds are the custom resource kinds the Console reads: the Flux
// deployment machinery plus the DIS platform resources. The served API version
// of each is resolved at runtime via the discovery RESTMapper, so the same
// binary keeps working if Azure Flux bumps a version; kinds whose CRD is not
// installed on a cluster are skipped by Sweep.
var TargetKinds = []schema.GroupKind{
	{Group: GroupKustomize, Kind: KindKustomization},
	{Group: GroupHelm, Kind: KindHelmRelease},
	{Group: GroupSource, Kind: KindOCIRepository},
	{Group: GroupSource, Kind: KindHelmRepository},
	{Group: GroupSource, Kind: KindHelmChart},
	{Group: GroupDISStorage, Kind: KindDatabaseServer},
	{Group: GroupDISStorage, Kind: KindDatabase},
	{Group: GroupDISVault, Kind: KindVault},
	{Group: GroupDISApplication, Kind: KindApplicationIdentity},
	{Group: GroupDISApim, Kind: KindApi},
	{Group: GroupDISApim, Kind: KindApiVersion},
	{Group: GroupDISApim, Kind: KindBackend},
}

// discoveryTTL bounds how often Sweep refreshes the discovery cache. It is long
// on purpose: re-discovery only matters to notice a newly installed Flux CRD (a
// rare extension-upgrade event), while each refresh re-fetches every API group's
// discovery document. A long TTL keeps the steady-state sweep to plain List
// calls and still picks new kinds up within the TTL.
const discoveryTTL = 10 * time.Minute

// Client lists Flux custom resources across all namespaces.
//
// A Client is not safe for concurrent use: Sweep reads and writes lastDiscovery
// without synchronization, so each Client must be swept from a single goroutine
// (the agent drives it from one poll loop).
type Client struct {
	dyn dynamic.Interface
	// mapper is held as the ResettableRESTMapper interface, not the concrete
	// deferred mapper, so tests can inject a fake; production still uses the
	// discovery-backed DeferredDiscoveryRESTMapper built in NewClient.
	mapper apimeta.ResettableRESTMapper
	// lastDiscovery is when the discovery cache was last reset; Sweep resets it
	// at most once per discoveryTTL (see the concurrency note above).
	lastDiscovery time.Time
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
// subset of the Flux controllers and DIS operators is present. Any other
// discovery/list failure
// (RBAC, auth, apiserver outage) aborts the sweep with an error so the caller
// keeps the previous snapshot instead of publishing a partial one.
//
// The discovery cache is reset at most once per discoveryTTL rather than on
// every sweep, so a newly installed Flux CRD is detected only on the next reset
// — up to discoveryTTL (~10 min) after it appears. Sweep is not safe for
// concurrent use; call it from a single goroutine.
func (c *Client) Sweep(ctx context.Context) ([]Resource, []error, error) {
	if time.Since(c.lastDiscovery) >= discoveryTTL {
		c.mapper.Reset()
		c.lastDiscovery = time.Now()
	}

	resources := make([]Resource, 0)
	var warnings []error

	for _, gk := range TargetKinds {
		mapping, err := c.mapper.RESTMapping(gk)
		if err != nil {
			// A kind that isn't installed is expected (optional source kinds,
			// DIS operators not deployed on this cluster); skip it. Any other
			// mapping error is a discovery/access failure and must abort the
			// sweep.
			if apimeta.IsNoMatchError(err) {
				warnings = append(warnings, fmt.Errorf("skip %s: not installed", gk))
				continue
			}
			return nil, warnings, fmt.Errorf("resolve %s: %w", gk, err)
		}
		nri := c.dyn.Resource(mapping.Resource).Namespace(metav1.NamespaceAll)
		// ResourceVersion "0" serves the list from the apiserver's watch cache
		// instead of a quorum read from etcd. The sub-second staleness that
		// allows is irrelevant to a status poller (the agent re-lists every poll
		// interval and the Console marks clusters stale only after minutes), and
		// it keeps the repeated fleet-wide lists off etcd.
		list, err := nri.List(ctx, metav1.ListOptions{ResourceVersion: "0"})
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
