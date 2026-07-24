// Package flux reads Flux and DIS custom resources — plus the label-filtered
// apps workloads — across all namespaces using the dynamic client and a
// discovery-backed RESTMapper, and normalizes their deployment status into a
// small, stable shape the API serves.
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
	"k8s.io/apimachinery/pkg/types"
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

// The built-in apps group and its workload kinds. An app's effective version
// is its container image, which manifest revisions cannot show (postBuild
// substitution can resolve a tag per cluster), so the sweep mirrors the
// long-running workloads too. Jobs/CronJobs/Pods are deliberately out: the
// console models what should be running, not runs.
const (
	GroupApps = "apps"

	KindDeployment  = "Deployment"
	KindStatefulSet = "StatefulSet"
	KindDaemonSet   = "DaemonSet"
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

// TargetKinds are the kinds the Console reads: the Flux deployment machinery,
// the DIS platform resources, and the apps workload kinds (label-filtered —
// see Sweep). The served API version of each is resolved at runtime via
// the discovery RESTMapper, so the same binary keeps working if Azure Flux
// bumps a version; kinds whose CRD is not installed on a cluster are skipped
// by Sweep (the apps kinds are built-in and always served).
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
	{Group: GroupApps, Kind: KindDeployment},
	{Group: GroupApps, Kind: KindStatefulSet},
	{Group: GroupApps, Kind: KindDaemonSet},
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

// helmManagedSelector is the second server-side workload filter: every object
// Helm renders carries the app.kubernetes.io/managed-by=Helm label (plus the
// meta.helm.sh release annotations Sweep resolves ownership from), while the
// kustomize-label selector cannot see chart objects — helm-controller applies
// them itself and stamps no kustomize labels.
const helmManagedSelector = labelManagedBy + "=" + managedByHelm

// Sweep lists every instance of each target kind across all namespaces and
// returns them as normalized resources. Kinds that are not installed are
// skipped (reported as warnings) so the sweep still succeeds when only a
// subset of the Flux controllers and DIS operators is present. Any other
// discovery/list failure
// (RBAC, auth, apiserver outage) aborts the sweep with an error so the caller
// keeps the previous snapshot instead of publishing a partial one.
//
// Workloads (the apps kinds) are mirrored only when a deployer labeled them:
// the kustomize-controller ownership label, or Helm's managed-by label. Both
// filters ride the lists as server-side label selectors, which keeps
// kube-system and Azure-managed add-ons out. A chart-created workload's
// annotations name its Helm release — not the HelmRelease CR, whose
// spec.releaseName/spec.targetNamespace change the release identity — so its
// appliedBy is resolved against the batch's HelmReleases after the loop;
// a release no swept HelmRelease accounts for (installed outside Flux) stays
// mirrored without an owner.
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
	// releases maps each HelmRelease's effective release identity to the CR;
	// helmOwned remembers which mirrored workloads (by index into resources)
	// wait for which release. Ownership is resolved after the kind loop, when
	// both sides of the join are complete regardless of TargetKinds order.
	releases := make(map[types.NamespacedName]AppliedBy)
	type helmRef struct {
		index   int
		release types.NamespacedName
	}
	var helmOwned []helmRef

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

		selectors := []string{""}
		// seen dedups the two workload lists by object identity: some charts
		// hardcode the managed-by label in their templates, so a
		// kustomize-applied object can match both selectors. The kustomize
		// pass runs first and wins — its labels name the true GitOps applier.
		var seen map[types.NamespacedName]bool
		if gk.Group == GroupApps {
			selectors = []string{LabelAppliedByName, helmManagedSelector}
			seen = make(map[types.NamespacedName]bool)
		}
		for _, selector := range selectors {
			// ResourceVersion "0" serves the list from the apiserver's watch
			// cache instead of a quorum read from etcd. The sub-second staleness
			// that allows is irrelevant to a status poller (the agent re-lists
			// every poll interval and the Console marks clusters stale only
			// after minutes), and it keeps the repeated fleet-wide lists off
			// etcd.
			opts := metav1.ListOptions{ResourceVersion: "0", LabelSelector: selector}
			list, err := nri.List(ctx, opts)
			if err != nil {
				return nil, warnings, fmt.Errorf("list %s: %w", gk, err)
			}
			for i := range list.Items {
				item := &list.Items[i]
				switch selector {
				case LabelAppliedByName:
					// The existence selector also matches a present-but-empty
					// label; only a named applier counts.
					if item.GetLabels()[LabelAppliedByName] == "" {
						continue
					}
					seen[types.NamespacedName{Namespace: item.GetNamespace(), Name: item.GetName()}] = true
				case helmManagedSelector:
					if seen[types.NamespacedName{Namespace: item.GetNamespace(), Name: item.GetName()}] {
						continue
					}
				}
				r, err := normalize(item)
				if err != nil {
					warnings = append(warnings, fmt.Errorf("normalize %s: %w", gk, err))
					continue
				}
				if gk.Group == GroupHelm && gk.Kind == KindHelmRelease {
					id, err := helmReleaseIdentity(item)
					if err != nil {
						warnings = append(warnings, fmt.Errorf("release identity %s/%s: %w", item.GetNamespace(), item.GetName(), err))
					} else {
						releases[id] = AppliedBy{Name: item.GetName(), Namespace: item.GetNamespace()}
					}
				}
				resources = append(resources, r)
				if selector == helmManagedSelector && r.AppliedBy == nil {
					if owner, ok := helmOwnerFrom(item.GetAnnotations()); ok {
						helmOwned = append(helmOwned, helmRef{index: len(resources) - 1, release: owner})
					}
				}
			}
		}
	}

	for _, w := range helmOwned {
		if ab, ok := releases[w.release]; ok {
			resources[w.index].AppliedBy = &ab
		}
	}
	return resources, warnings, nil
}
