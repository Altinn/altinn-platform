package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/central"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
)

// Artifact classes derived from the OCI URL path — the only identity a
// base-layer artifact carries (neither the CRs nor the artifacts have product
// or team labels). The fleet's conventions: product syncroots live at
// <product>/syncroot (the dis_products_syncroot_multitenancy fluxConfiguration
// module), admin syncroots at <ns>/syncroot-admin, gitops-manifests packages
// at manifests/infra/<pkg>, and operator kustomize configs at
// dis/kustomize/<operator>.
const (
	ClassProductSyncroot = "product-syncroot"
	ClassAdminSyncroot   = "admin-syncroot"
	ClassInfra           = "infra"
	ClassOperator        = "operator"
	ClassOther           = "other"
)

// classifyArtifact derives (class, owner) from an OCIRepository URL. Owner is
// the product for the syncroot classes and the package/operator name for the
// platform classes; empty when the URL matches no convention.
func classifyArtifact(url string) (class, owner string) {
	segs := strings.Split(strings.Trim(strings.TrimPrefix(url, "oci://"), "/"), "/")
	if len(segs) < 2 {
		return ClassOther, ""
	}
	segs = segs[1:] // drop the registry host
	last := len(segs) - 1
	switch {
	case len(segs) >= 2 && segs[last] == "syncroot":
		return ClassProductSyncroot, strings.Join(segs[:last], "/")
	case len(segs) >= 2 && segs[last] == "syncroot-admin":
		return ClassAdminSyncroot, strings.Join(segs[:last], "/")
	case len(segs) >= 3 && segs[0] == "manifests" && segs[1] == "infra":
		return ClassInfra, strings.Join(segs[2:], "/")
	case len(segs) >= 3 && segs[0] == "dis" && segs[1] == "kustomize":
		return ClassOperator, strings.Join(segs[2:], "/")
	}
	return ClassOther, ""
}

// artifactKustomization is a Kustomization deploying an artifact, as embedded
// in the artifacts view: identity plus the applied revision and health, enough
// for a version matrix without a second request.
type artifactKustomization struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Revision  string `json:"revision,omitempty"`
	Ready     string `json:"ready"`
	Reason    string `json:"reason,omitempty"`
	Suspended bool   `json:"suspended"`
}

// artifactView is one OCIRepository row served as a base-layer artifact.
// Revision is the fetched artifact (tag@sha256:… — the digest is the real
// version; the tags are mutable environment/ring tags), while each attached
// kustomization's revision is what is actually applied, so a mismatch between
// the two is a rollout in flight.
type artifactView struct {
	Cluster        string                  `json:"cluster"`
	Namespace      string                  `json:"namespace"`
	Name           string                  `json:"name"`
	URL            string                  `json:"url"`
	Class          string                  `json:"class"`
	Owner          string                  `json:"owner,omitempty"`
	Revision       string                  `json:"revision,omitempty"`
	OriginRevision string                  `json:"originRevision,omitempty"`
	OriginSource   string                  `json:"originSource,omitempty"`
	Ready          string                  `json:"ready"`
	Reason         string                  `json:"reason,omitempty"`
	Suspended      bool                    `json:"suspended"`
	Kustomizations []artifactKustomization `json:"kustomizations"`
}

type artifactsResponse struct {
	Count     int            `json:"count"`
	Artifacts []artifactView `json:"artifacts"`
}

// handleArtifacts serves the base-layer overview: every OCIRepository in the
// mirror (one cluster when ?cluster= is set), classified by URL, with the
// Kustomizations that build from it attached. ?class= filters to one class.
func (s *Server) handleArtifacts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cluster, class := q.Get("cluster"), q.Get("class")

	repos, err := s.store.List(r.Context(), cluster, flux.KindOCIRepository, "", "")
	if err != nil {
		s.fail(w, "artifacts", err)
		return
	}
	kusts, err := s.store.List(r.Context(), cluster, flux.KindKustomization, "", "")
	if err != nil {
		s.fail(w, "artifacts", err)
		return
	}

	byRepo := map[string][]artifactKustomization{}
	for _, k := range kusts {
		if k.SourceRef == nil || k.SourceRef.Kind != flux.KindOCIRepository {
			continue
		}
		key := repoKey(k.Cluster, k.SourceRef.Namespace, k.SourceRef.Name)
		byRepo[key] = append(byRepo[key], artifactKustomization{
			Name:      k.Name,
			Namespace: k.Namespace,
			Revision:  k.Revision,
			Ready:     k.Ready,
			Reason:    k.Reason,
			Suspended: k.Suspended,
		})
	}

	artifacts := []artifactView{}
	for _, repo := range repos {
		c, owner := classifyArtifact(repo.SourceURL)
		if class != "" && !strings.EqualFold(c, class) {
			continue
		}
		ks := byRepo[repoKey(repo.Cluster, repo.Namespace, repo.Name)]
		if ks == nil {
			ks = []artifactKustomization{}
		}
		artifacts = append(artifacts, artifactView{
			Cluster:        repo.Cluster,
			Namespace:      repo.Namespace,
			Name:           repo.Name,
			URL:            repo.SourceURL,
			Class:          c,
			Owner:          owner,
			Revision:       repo.Revision,
			OriginRevision: repo.OriginRevision,
			OriginSource:   repo.OriginSource,
			Ready:          repo.Ready,
			Reason:         repo.Reason,
			Suspended:      repo.Suspended,
			Kustomizations: ks,
		})
	}
	writeJSON(w, http.StatusOK, artifactsResponse{Count: len(artifacts), Artifacts: artifacts})
}

// repoKey identifies an OCIRepository within a cluster; a Kustomization's
// projected sourceRef namespace is already default-resolved, so the join is a
// plain key match.
func repoKey(cluster, namespace, name string) string {
	return cluster + "|" + namespace + "|" + name
}

// inventoryEntryView is one applied object from a Kustomization's inventory,
// expanded from Flux's compact id. Resource carries the mirrored row when the
// entry is a kind the agent sweeps (DIS CRs, HelmReleases, nested Flux
// objects); null for everything else (Deployments, Services, ...).
type inventoryEntryView struct {
	Group     string            `json:"group,omitempty"`
	Kind      string            `json:"kind"`
	Namespace string            `json:"namespace,omitempty"`
	Name      string            `json:"name"`
	Version   string            `json:"version,omitempty"`
	Resource  *central.Resource `json:"resource,omitempty"`
}

type inventoryResponse struct {
	Cluster   string               `json:"cluster"`
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Revision  string               `json:"revision,omitempty"`
	Count     int                  `json:"count"`
	Entries   []inventoryEntryView `json:"entries"`
}

// handleKustomizationInventory expands one Kustomization's applied-object set
// (status.inventory) — the full deployment tree of a syncroot or infra package
// — enriched with the mirrored status of every entry the agent sweeps. Empty
// entries mean the Kustomization has not recorded an inventory yet.
func (s *Server) handleKustomizationInventory(w http.ResponseWriter, r *http.Request) {
	cluster := r.PathValue("cluster")
	res, err := s.store.Get(r.Context(),
		cluster, flux.KindKustomization, r.PathValue("namespace"), r.PathValue("name"))
	if errors.Is(err, central.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody("kustomization not found"))
		return
	}
	if err != nil {
		s.fail(w, "inventory", err)
		return
	}

	entries := []inventoryEntryView{}
	if len(res.Inventory) > 0 {
		// One cluster-wide list indexes every mirrored row for enrichment; the
		// payload equals what /api/resources?cluster= already serves.
		rows, err := s.store.List(r.Context(), cluster, "", "", "")
		if err != nil {
			s.fail(w, "inventory", err)
			return
		}
		byKey := make(map[string]*central.Resource, len(rows))
		for i := range rows {
			byKey[resourceKey(rows[i].Kind, rows[i].Namespace, rows[i].Name)] = &rows[i]
		}
		for _, e := range res.Inventory {
			v := entryView(e)
			v.Resource = byKey[resourceKey(v.Kind, v.Namespace, v.Name)]
			entries = append(entries, v)
		}
	}

	writeJSON(w, http.StatusOK, inventoryResponse{
		Cluster:   cluster,
		Namespace: res.Namespace,
		Name:      res.Name,
		Revision:  res.Revision,
		Count:     len(entries),
		Entries:   entries,
	})
}

// entryView expands Flux's compact inventory id (`<namespace>_<name>_<group>_
// <kind>`; namespace empty for cluster-scoped objects, group empty for the
// core API). Kubernetes names, namespaces and groups cannot contain
// underscores, so the split is unambiguous; a malformed id (never produced by
// kustomize-controller) degrades to the raw id in Name so it stays visible.
func entryView(e flux.InventoryEntry) inventoryEntryView {
	parts := strings.Split(e.ID, "_")
	if len(parts) != 4 {
		return inventoryEntryView{Name: e.ID, Version: e.Version}
	}
	return inventoryEntryView{
		Namespace: parts[0],
		Name:      parts[1],
		Group:     parts[2],
		Kind:      parts[3],
		Version:   e.Version,
	}
}

// resourceKey identifies a mirrored resource within a cluster; kind is folded
// because inventory ids carry CRD-spelled kinds while rows store them as swept.
func resourceKey(kind, namespace, name string) string {
	return strings.ToLower(kind) + "|" + namespace + "|" + name
}
