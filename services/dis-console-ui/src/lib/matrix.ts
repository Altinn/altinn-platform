import type { Resource } from '../api/types';
import {
  ENV_ORDER,
  STATUS_SEVERITY,
  envOf,
  isApp,
  isWorkloadKind,
  shortRev,
  statusOf,
  tenantOf,
  type DeployStatus,
  type Environment,
} from './flux';

export interface MatrixCell {
  env: string;
  status: DeployStatus;
  resource?: Resource;
  /** HelmReleases applied by this row's Kustomization, folded into the cell. */
  children?: Resource[];
  /** Set when more than one cluster maps to the same (row, env) slot. */
  conflict?: Resource[];
}

export function worstOf(a: DeployStatus, b: DeployStatus): DeployStatus {
  return STATUS_SEVERITY[b] > STATUS_SEVERITY[a] ? b : a;
}

export interface MatrixRow {
  /** Stable identity across environments: kind|namespace|name. */
  key: string;
  kind: string;
  namespace: string;
  name: string;
  tenant: string;
  cells: Record<string, MatrixCell>;
  anyFailed: boolean;
  /** Environments disagree on revision (i.e. a promotion is mid-flight). */
  drift: boolean;
}

export interface Matrix {
  /** Columns, in promotion order (all known envs, plus any unexpected ones). */
  envs: string[];
  rows: MatrixRow[];
}

export interface BuildOptions {
  tenant?: string;
  namespace?: string;
  /** Filter to one kind (shown flat, azapi roots included); when unset, only
   *  apps are shown — our Kustomizations/HelmReleases, see `isApp`. */
  kind?: string;
}

/** Every tenant present in the resource set (cluster prefix), sorted. */
export function tenantsOf(resources: Resource[]): string[] {
  return [...new Set(resources.map((r) => tenantOf(r.cluster)))].sort();
}

/** Namespaces present, optionally scoped to one tenant, sorted. */
export function namespacesOf(resources: Resource[], tenant?: string): string[] {
  const scoped = tenant ? resources.filter((r) => tenantOf(r.cluster) === tenant) : resources;
  return [...new Set(scoped.map((r) => r.namespace))].sort();
}

/** Kinds present, optionally scoped to one tenant, sorted. */
export function kindsOf(resources: Resource[], tenant?: string): string[] {
  const scoped = tenant ? resources.filter((r) => tenantOf(r.cluster) === tenant) : resources;
  return [...new Set(scoped.map((r) => r.kind))].sort();
}

function rowKey(r: Resource): string {
  return `${r.kind}|${r.namespace}|${r.name}`;
}

/**
 * Shape the flat /api/resources list into an apps × environments matrix:
 * one row per Flux resource identity (kind+namespace+name), one column per
 * environment derived from the cluster id. Row identity never includes the
 * cluster/environment — that is the column axis.
 */
export function buildMatrix(resources: Resource[], opts: BuildOptions = {}): Matrix {
  const scoped = resources.filter(
    (r) =>
      (!opts.tenant || tenantOf(r.cluster) === opts.tenant) &&
      (!opts.namespace || r.namespace === opts.namespace) &&
      (opts.kind ? r.kind === opts.kind : isApp(r) || (isWorkloadKind(r.kind) && !!r.appliedBy)),
  );

  // In the default apps view, a HelmRelease or workload applied by an app row
  // that is itself in the data folds into that row (its health rolls up into
  // the cell) instead of appearing as a row of its own; a workload whose
  // applier is NOT an app (a syncroot root) becomes its own row — the
  // dis-console-on-admin pattern, and the shape a future DisApp formalizes.
  // An explicit kind filter shows resources flat. Kustomizations never fold —
  // they also carry the owner labels (everything is ultimately applied by the
  // syncroot root), so folding them would collapse the whole matrix.
  const fold = !opts.kind;
  const appKeys = new Set(scoped.filter((r) => isApp(r)).map((r) => rowKey(r)));
  const ownerKeyOf = (r: Resource): string | undefined => {
    if (!fold || !r.appliedBy) return undefined;
    const kust = `Kustomization|${r.appliedBy.namespace}|${r.appliedBy.name}`;
    if (r.kind === 'HelmRelease') return appKeys.has(kust) ? kust : undefined;
    if (!isWorkloadKind(r.kind)) return undefined;
    if (appKeys.has(kust)) return kust;
    const hr = `HelmRelease|${r.appliedBy.namespace}|${r.appliedBy.name}`;
    return appKeys.has(hr) ? hr : undefined;
  };

  const byRow = new Map<string, MatrixRow>();
  const foldedChildren: Resource[] = [];
  for (const r of scoped) {
    if (ownerKeyOf(r)) {
      foldedChildren.push(r);
      continue;
    }
    const key = rowKey(r);
    const env = envOf(r.cluster);
    let row = byRow.get(key);
    if (!row) {
      row = {
        key,
        kind: r.kind,
        namespace: r.namespace,
        name: r.name,
        tenant: tenantOf(r.cluster),
        cells: {},
        anyFailed: false,
        drift: false,
      };
      byRow.set(key, row);
    }
    const existing = row.cells[env];
    if (existing?.resource) {
      // Two clusters resolve to the same environment slot — record both rather
      // than letting the last write silently win.
      existing.conflict = [...(existing.conflict ?? [existing.resource]), r];
    } else {
      row.cells[env] = { ...row.cells[env], env, status: statusOf(r), resource: r };
    }
  }

  for (const r of foldedChildren) {
    const row = byRow.get(ownerKeyOf(r)!);
    if (!row) continue;
    const env = envOf(r.cluster);
    const cell = (row.cells[env] ??= { env, status: 'absent' });
    cell.children = [...(cell.children ?? []), r];
  }

  // Columns: the environments this tenant actually has clusters in (any
  // mirrored resource counts — every swept cluster carries at least its
  // syncroot pair), in promotion order, then any unexpected envs. Presence is
  // computed tenant-wide, not from the kind/namespace scope, so the columns
  // stay stable while filtering — and a tenant like admin gets its test/prod
  // columns instead of six empty TE stages.
  const tenantScoped = opts.tenant
    ? resources.filter((r) => tenantOf(r.cluster) === opts.tenant)
    : resources;
  const present = new Set(tenantScoped.map((r) => envOf(r.cluster)));
  const extraEnvs = [...present]
    .filter((e) => e && !ENV_ORDER.includes(e as Environment))
    .sort();
  const envs = [...ENV_ORDER.filter((e) => present.has(e)), ...extraEnvs];

  const rows = [...byRow.values()];
  for (const row of rows) {
    const cells = Object.values(row.cells);
    for (const cell of cells) {
      let status: DeployStatus = cell.resource ? statusOf(cell.resource) : 'absent';
      for (const child of cell.children ?? []) status = worstOf(status, statusOf(child));
      cell.status = status;
    }
    row.anyFailed = cells.some((c) => c.status === 'failed');
    const revs = new Set(cells.map((c) => shortRev(c.resource?.revision)).filter(Boolean));
    row.drift = revs.size > 1;
  }

  rows.sort((a, b) => {
    if (a.anyFailed !== b.anyFailed) return a.anyFailed ? -1 : 1;
    if (a.namespace !== b.namespace) return a.namespace.localeCompare(b.namespace);
    return a.name.localeCompare(b.name);
  });

  return { envs, rows };
}
