// Types mirror the dis-console fleet API JSON exactly (services/dis-console).
// See internal/flux/normalize.go (Resource) and internal/central/central.go
// (Cluster, Resource), and internal/api/server.go (summary + envelopes).

export type ReadyState = 'True' | 'False' | 'Unknown';

/** One container image from a workload's pod template (schema v5 sweep). */
export interface ContainerImage {
  container: string;
  image: string;
}

/** A normalized Flux resource as served by /api/resources. */
export interface Resource {
  kind: string;
  apiVersion: string;
  namespace: string;
  name: string;
  ready: ReadyState;
  reason?: string;
  message?: string;
  revision?: string;
  suspended: boolean;
  generation?: number;
  observedGeneration?: number;
  /** RFC3339 timestamp of the Ready condition's last transition. */
  lastTransition?: string;
  /** Azure ARM resource id — for DIS resources whose status surfaces it. */
  azureResourceId?: string;
  /** Declared container images (pod template) — for workload kinds. */
  images?: ContainerImage[];
  /** The resource this one is grouped under (e.g. a Database's DatabaseServer). */
  parent?: { kind: string; name: string };
  /** The Kustomization that applied this object (from the kustomize labels). */
  appliedBy?: { name: string; namespace: string };
  /** A Kustomization's Flux source — the join key to its artifact. */
  sourceRef?: { kind: string; name: string; namespace?: string };
  /** A source kind's artifact URL (OCIRepository/HelmRepository spec.url). */
  sourceUrl?: string;
  /** The artifact's origin git annotations (flux push --revision/--source). */
  originRevision?: string;
  originSource?: string;
  /** The full k8s object — only present on the detail endpoint. */
  raw?: unknown;
  /** The cluster this resource was mirrored from (e.g. "acme_at23"). */
  cluster: string;
}

/** One status transition from a resource's history (detail endpoint only). */
export interface StatusEvent {
  ready: ReadyState;
  reason?: string;
  revision?: string;
  /** RFC3339 timestamp the transition was observed. */
  observedAt: string;
}

/** The detail endpoint's shape: the resource + its status-event history
 *  (newest first, bounded by the server's retention window). */
export interface ResourceDetail extends Resource {
  history?: StatusEvent[];
}

/** A tenant's sync status, served by /api/clusters. */
export interface Cluster {
  cluster: string;
  environment?: string;
  lastSweepAt: string;
  lastSyncedAt: string;
  agentVersion?: string;
  schemaVersion: number;
  resourceCount: number;
  stale: boolean;
}

export interface KindSummary {
  kind: string;
  total: number;
  ready: number;
  notReady: number;
  unknown: number;
  suspended: number;
}

export interface Summary {
  cluster?: string;
  total: number;
  kinds: KindSummary[];
}

export interface ClustersResponse {
  count: number;
  clusters: Cluster[];
}

export interface ResourcesResponse {
  count: number;
  resources: Resource[];
}

/** A Kustomization deploying an artifact, embedded in /api/artifacts. */
export interface ArtifactKustomization {
  name: string;
  namespace: string;
  /** The applied revision — differs from the artifact's while a rollout is in flight. */
  revision?: string;
  ready: ReadyState;
  reason?: string;
  suspended: boolean;
}

/** One base-layer OCI artifact (an OCIRepository row) from /api/artifacts. */
export interface Artifact {
  cluster: string;
  namespace: string;
  name: string;
  url: string;
  /** product-syncroot | admin-syncroot | infra | operator | other. */
  class: string;
  owner?: string;
  /** The fetched artifact, tag@sha256:… — the digest is the real version. */
  revision?: string;
  originRevision?: string;
  originSource?: string;
  ready: ReadyState;
  reason?: string;
  suspended: boolean;
  kustomizations: ArtifactKustomization[];
}

export interface ArtifactsResponse {
  count: number;
  artifacts: Artifact[];
}

/** One applied object from a Kustomization's inventory; resource is the
 *  mirrored row when the entry is a kind the agent sweeps, null otherwise. */
export interface InventoryEntry {
  group?: string;
  kind: string;
  namespace?: string;
  name: string;
  version?: string;
  resource?: Resource | null;
}

export interface InventoryResponse {
  cluster: string;
  namespace: string;
  name: string;
  revision?: string;
  count: number;
  entries: InventoryEntry[];
}
