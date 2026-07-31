import type { ContainerImage, Resource } from '../api/types';
import { deployedBySyncroot, type ArtifactRow } from './artifacts';
import { envOf, isWorkloadKind, statusOf, tenantOf, type DeployStatus } from './flux';

export { WORKLOAD_KINDS, isWorkloadKind } from './flux';

/** A workload's primary container image: the container named like the
 *  workload, else the first — mirrors how the fleet API will pick the
 *  workload's revision. */
export function primaryImage(
  images: ContainerImage[] | undefined,
  workloadName: string,
): string | undefined {
  if (!images || images.length === 0) return undefined;
  return (images.find((i) => i.container === workloadName) ?? images[0]).image;
}

export interface ImageRef {
  repo: string;
  tag?: string;
  digest?: string;
}

/** Split an image reference into repo / tag / digest. Registry ports are not
 *  tags: the colon only counts after the last path slash. */
export function parseImageRef(image: string): ImageRef {
  let rest = image;
  let digest: string | undefined;
  const at = rest.indexOf('@');
  if (at >= 0) {
    digest = rest.slice(at + 1);
    rest = rest.slice(0, at);
  }
  const slash = rest.lastIndexOf('/');
  const colon = rest.lastIndexOf(':');
  if (colon > slash) {
    return { repo: rest.slice(0, colon), tag: rest.slice(colon + 1), digest };
  }
  return { repo: rest, digest };
}

/** The label to show for an image: its tag, else a short digest, else the
 *  implicit `latest`. */
export function imageLabel(image: string): string {
  const ref = parseImageRef(image);
  if (ref.tag) return ref.tag;
  if (ref.digest) return ref.digest.replace(/^sha256:/, '').slice(0, 12);
  return 'latest';
}

export interface WorkloadCell {
  status: DeployStatus;
  /** container name → full image reference. */
  images: Record<string, string>;
}

export interface WorkloadRow {
  key: string;
  kind: string;
  namespace: string;
  name: string;
  /** Union of container names across environments, sorted. */
  containers: string[];
  cells: Partial<Record<string, WorkloadCell>>;
  /** Containers whose image differs between environments. */
  driftContainers: Set<string>;
}

/** A workload row labeled with what applies it, for aggregate views. */
export interface AppWorkloadRow extends WorkloadRow {
  app?: string;
}

export interface AppIdentity {
  namespace: string;
  name: string;
  tenant: string;
}

function addCell(rows: Map<string, AppWorkloadRow>, r: Resource, env: string): AppWorkloadRow {
  const key = `${r.kind}|${r.namespace}|${r.name}`;
  let row = rows.get(key);
  if (!row) {
    row = {
      key,
      kind: r.kind,
      namespace: r.namespace,
      name: r.name,
      containers: [],
      cells: {},
      driftContainers: new Set(),
      app: r.appliedBy?.name,
    };
    rows.set(key, row);
  }
  row.cells[env] = {
    status: statusOf(r),
    images: Object.fromEntries((r.images ?? []).map((i) => [i.container, i.image])),
  };
  return row;
}

function finalize(rows: Map<string, AppWorkloadRow>): AppWorkloadRow[] {
  for (const row of rows.values()) {
    const containers = new Set<string>();
    for (const cell of Object.values(row.cells)) {
      for (const c of Object.keys(cell?.images ?? {})) containers.add(c);
    }
    row.containers = [...containers].sort();
    for (const c of row.containers) {
      const distinct = new Set(
        Object.values(row.cells)
          .map((cell) => cell?.images[c])
          .filter((img): img is string => Boolean(img)),
      );
      if (distinct.size > 1) row.driftContainers.add(c);
    }
  }
  return [...rows.values()].sort((a, b) => {
    if (a.namespace !== b.namespace) return a.namespace.localeCompare(b.namespace);
    return a.name.localeCompare(b.name);
  });
}

/**
 * The workloads one app declares, as workload × environment rows of container
 * images — matched via `appliedBy` (name + namespace of the applier). Works
 * for Kustomization apps today; HelmRelease apps light up once the fleet API
 * projects Helm ownership onto chart-created workloads. `extraOwners` are
 * additional applier identities belonging to the same app row — the
 * HelmReleases folded into a Kustomization's row, whose chart workloads carry
 * the HR as their applier.
 */
export function workloadsOf(
  resources: Resource[],
  app: AppIdentity,
  envs: string[],
  extraOwners: { name: string; namespace: string }[] = [],
): WorkloadRow[] {
  const owners = new Set([
    `${app.namespace}|${app.name}`,
    ...extraOwners.map((o) => `${o.namespace}|${o.name}`),
  ]);
  const rows = new Map<string, AppWorkloadRow>();
  for (const r of resources) {
    if (!isWorkloadKind(r.kind)) continue;
    if (!r.appliedBy || !owners.has(`${r.appliedBy.namespace}|${r.appliedBy.name}`)) continue;
    if (tenantOf(r.cluster) !== app.tenant) continue;
    const env = envOf(r.cluster);
    if (!envs.includes(env)) continue;
    addCell(rows, r, env);
  }
  return finalize(rows);
}

/** A workload app's own row (the app IS the workload) — for the release
 *  drawer of rows that are themselves Deployments/StatefulSets/DaemonSets. */
export function selfWorkload(
  current: Partial<Record<string, Resource>>,
  envs: string[],
): AppWorkloadRow[] {
  const rows = new Map<string, AppWorkloadRow>();
  for (const env of envs) {
    const r = current[env];
    if (r && isWorkloadKind(r.kind)) addCell(rows, r, env);
  }
  return finalize(rows);
}

/**
 * Every workload a syncroot deploys, across environments — read straight from
 * the appliedBy closure, so workloads applied *directly by the root
 * Kustomization* (no app of their own — e.g. dis-console on the admin
 * clusters) appear too. `app` carries the applier's name.
 */
export function syncrootWorkloads(
  resources: Resource[],
  row: Pick<ArtifactRow, 'cells'>,
  envs: string[],
): AppWorkloadRow[] {
  const rows = new Map<string, AppWorkloadRow>();
  for (const env of envs) {
    const cell = row.cells[env];
    if (!cell) continue;
    for (const r of deployedBySyncroot(resources, cell.artifact.cluster, cell.artifact.kustomizations)) {
      if (isWorkloadKind(r.kind)) addCell(rows, r, env);
    }
  }
  return finalize(rows);
}
