import type { Artifact, Resource } from '../api/types';
import {
  ENV_ORDER,
  STATUS_SEVERITY,
  envOf,
  tenantOf,
  type DeployStatus,
  type Environment,
} from './flux';

/** Display labels for the artifact classes /api/artifacts derives from OCI URLs. */
export const CLASS_LABELS: Record<string, string> = {
  'product-syncroot': 'Product syncroot',
  'admin-syncroot': 'Admin syncroot',
  infra: 'Infra',
  operator: 'Operator',
  other: 'Other',
};

const CLASS_ORDER = ['product-syncroot', 'admin-syncroot', 'infra', 'operator', 'other'];

export function classLabel(cls: string): string {
  return CLASS_LABELS[cls] ?? cls;
}

/** The sha256 digest (shortened) of an artifact revision like `tag@sha256:…` —
 *  the digest is the real version; the tag is a mutable environment/ring tag. */
export function shortDigest(revision: string | undefined): string {
  if (!revision) return '';
  const m = revision.match(/sha256:([0-9a-f]+)/i);
  if (m) return m[1].slice(0, 7);
  return revision.includes('@') ? (revision.split('@').pop() ?? revision) : revision;
}

/** The tag part of an artifact revision (`at23@sha256:…` → `at23`). */
export function artifactTag(revision: string | undefined): string {
  if (!revision) return '';
  const i = revision.indexOf('@');
  return i > 0 ? revision.slice(0, i) : '';
}

/** Coarse status of an artifact row (same mapping as resources). */
export function artifactStatus(a: Pick<Artifact, 'ready' | 'suspended'>): DeployStatus {
  if (a.suspended) return 'suspended';
  switch (a.ready) {
    case 'True':
      return 'healthy';
    case 'False':
      return 'failed';
    case 'Unknown':
      return 'reconciling';
    default:
      return 'unknown';
  }
}

/** True when a Kustomization has not yet applied the artifact's digest — a
 *  rollout in flight (or wedged). */
export function inFlight(a: Artifact): boolean {
  const digest = shortDigest(a.revision);
  if (!digest) return false;
  return a.kustomizations.some((k) => k.revision && shortDigest(k.revision) !== digest);
}

/** A syncroot as a "project" card: the namespaces it deploys into and the
 *  clusters/environments it covers, with its worst status across them. */
export interface SyncrootSummary {
  key: string;
  class: string;
  owner?: string;
  namespace: string;
  name: string;
  tenant: string;
  envs: string[];
  namespaces: string[];
  worst: DeployStatus;
  rolling: boolean;
}

export function syncrootSummaries(artifacts: Artifact[], resources: Resource[]): SyncrootSummary[] {
  const matrix = buildArtifactMatrix(syncrootArtifacts(artifacts));
  return matrix.rows.map((row) => {
    const cells = matrix.envs.map((e) => row.cells[e]).filter(Boolean);
    const namespaces = new Set<string>();
    let worst: DeployStatus = 'absent';
    let rolling = false;
    for (const cell of cells) {
      const status = artifactStatus(cell.artifact);
      if (STATUS_SEVERITY[status] > STATUS_SEVERITY[worst]) worst = status;
      rolling = rolling || cell.inFlight;
      for (const r of deployedBySyncroot(resources, cell.artifact.cluster, cell.artifact.kustomizations)) {
        namespaces.add(r.namespace);
      }
    }
    return {
      key: row.key,
      class: row.class,
      owner: row.owner,
      namespace: row.namespace,
      name: row.name,
      tenant: cells.length > 0 ? tenantOf(cells[0].artifact.cluster) : '',
      envs: matrix.envs.filter((e) => row.cells[e]),
      namespaces: [...namespaces].sort(),
      worst,
      rolling,
    };
  });
}

export interface ArtifactCell {
  artifact: Artifact;
  inFlight: boolean;
  /** Set when more than one cluster maps to the same (row, env) slot. */
  conflict?: Artifact[];
}

export interface ArtifactRow {
  /** Stable identity across environments: class|owner|namespace|name. */
  key: string;
  class: string;
  owner?: string;
  namespace: string;
  name: string;
  cells: Record<string, ArtifactCell>;
}

export interface ArtifactMatrix {
  envs: string[];
  rows: ArtifactRow[];
}

export interface ArtifactBuildOptions {
  tenant?: string;
  class?: string;
}

/** Tenants present in the artifact set (cluster prefix), sorted. */
export function artifactTenantsOf(artifacts: Artifact[]): string[] {
  return [...new Set(artifacts.map((a) => tenantOf(a.cluster)))].sort();
}

/** Environments with artifacts for a tenant, in promotion order. */
export function artifactEnvsOf(artifacts: Artifact[], tenant?: string): string[] {
  const present = new Set(
    artifacts
      .filter((a) => !tenant || tenantOf(a.cluster) === tenant)
      .map((a) => envOf(a.cluster)),
  );
  const known = ENV_ORDER.filter((e) => present.has(e));
  const extra = [...present].filter((e) => e && !ENV_ORDER.includes(e as Environment)).sort();
  return [...known, ...extra];
}

const SYNCROOT_CLASSES = ['product-syncroot', 'admin-syncroot'];

/** Only the syncroot-class artifacts (product + admin syncroots). */
export function syncrootArtifacts(artifacts: Artifact[]): Artifact[] {
  return artifacts.filter((a) => SYNCROOT_CLASSES.includes(a.class));
}

/** The syncroot artifacts of one cluster — the roots of its deployment tree. */
export function syncrootsFor(artifacts: Artifact[], cluster: string): Artifact[] {
  return syncrootArtifacts(artifacts)
    .filter((a) => a.cluster === cluster)
    .sort((a, b) => a.name.localeCompare(b.name));
}

/**
 * Every swept resource a syncroot deployed on one cluster: the transitive
 * closure over `appliedBy`, seeded with the syncroot's root Kustomizations.
 * The kustomize controller stamps appliedBy on everything it applies, so app
 * Kustomizations chain to the root and their CRs chain to them; the roots
 * themselves (Azure-managed, no appliedBy) are not part of the result.
 */
export function deployedBySyncroot(
  resources: Resource[],
  cluster: string,
  roots: { name: string; namespace: string }[],
): Resource[] {
  const inCluster = resources.filter((r) => r.cluster === cluster);
  const owners = new Set(roots.map((r) => `${r.namespace}|${r.name}`));
  const added = new Set<string>();
  const out: Resource[] = [];

  let grew = true;
  while (grew) {
    grew = false;
    for (const r of inCluster) {
      const id = `${r.kind}|${r.namespace}|${r.name}`;
      if (added.has(id) || !r.appliedBy) continue;
      if (!owners.has(`${r.appliedBy.namespace}|${r.appliedBy.name}`)) continue;
      added.add(id);
      out.push(r);
      if (r.kind === 'Kustomization') owners.add(`${r.namespace}|${r.name}`);
      grew = true;
    }
  }

  return out.sort(
    (a, b) =>
      a.kind.localeCompare(b.kind) ||
      a.namespace.localeCompare(b.namespace) ||
      a.name.localeCompare(b.name),
  );
}

/** Classes present, optionally scoped to one tenant, in canonical order. */
export function artifactClassesOf(artifacts: Artifact[], tenant?: string): string[] {
  const scoped = tenant ? artifacts.filter((a) => tenantOf(a.cluster) === tenant) : artifacts;
  const present = new Set(scoped.map((a) => a.class));
  return [
    ...CLASS_ORDER.filter((c) => present.has(c)),
    ...[...present].filter((c) => !CLASS_ORDER.includes(c)).sort(),
  ];
}

/**
 * Shape the flat /api/artifacts list into artifacts × environments: one row
 * per artifact identity (class+owner+namespace+name), one column per
 * environment derived from the cluster id — the base-layer version matrix.
 */
export function buildArtifactMatrix(
  artifacts: Artifact[],
  opts: ArtifactBuildOptions = {},
): ArtifactMatrix {
  const scoped = artifacts.filter(
    (a) =>
      (!opts.tenant || tenantOf(a.cluster) === opts.tenant) &&
      (!opts.class || a.class === opts.class),
  );

  const byRow = new Map<string, ArtifactRow>();
  for (const a of scoped) {
    const key = `${a.class}|${a.owner ?? ''}|${a.namespace}|${a.name}`;
    const env = envOf(a.cluster);
    let row = byRow.get(key);
    if (!row) {
      row = { key, class: a.class, owner: a.owner, namespace: a.namespace, name: a.name, cells: {} };
      byRow.set(key, row);
    }
    const existing = row.cells[env];
    if (existing) {
      existing.conflict = [...(existing.conflict ?? [existing.artifact]), a];
    } else {
      row.cells[env] = { artifact: a, inFlight: inFlight(a) };
    }
  }

  // Same column rule as buildMatrix: only environments this tenant has
  // clusters in, in promotion order (admin tenants get test/prod, not six
  // empty TE stages).
  const tenantScoped = opts.tenant
    ? artifacts.filter((a) => tenantOf(a.cluster) === opts.tenant)
    : artifacts;
  const present = new Set(tenantScoped.map((a) => envOf(a.cluster)));
  const extraEnvs = [...present]
    .filter((e) => e && !ENV_ORDER.includes(e as Environment))
    .sort();
  const envs = [...ENV_ORDER.filter((e) => present.has(e)), ...extraEnvs];

  const rows = [...byRow.values()].sort((a, b) => {
    const ca = CLASS_ORDER.indexOf(a.class);
    const cb = CLASS_ORDER.indexOf(b.class);
    if (ca !== cb) return (ca === -1 ? CLASS_ORDER.length : ca) - (cb === -1 ? CLASS_ORDER.length : cb);
    if ((a.owner ?? '') !== (b.owner ?? '')) return (a.owner ?? '').localeCompare(b.owner ?? '');
    return a.name.localeCompare(b.name);
  });

  return { envs, rows };
}
