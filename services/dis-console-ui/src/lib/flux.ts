import type { Resource } from '../api/types';

/** Environments in promotion order — the columns of the deployment matrix. */
// Promotion order for stage columns: the TE ring (at → tt → yt → production),
// then the admin clusters' own two-stage ring (test → prod). The two sets
// never co-occur within a tenant — column builders render only the
// environments a tenant actually has clusters in.
export const ENV_ORDER = [
  'at22',
  'at23',
  'at24',
  'tt02',
  'yt01',
  'production',
  'test',
  'prod',
] as const;
export type Environment = (typeof ENV_ORDER)[number];

const ENV_LABELS: Record<string, string> = {
  at22: 'AT22',
  at23: 'AT23',
  at24: 'AT24',
  tt02: 'TT02',
  yt01: 'YT01',
  production: 'Production',
  test: 'Test',
  prod: 'Prod',
};

/** Human-facing column label for an environment id. */
export function envLabel(env: string): string {
  return ENV_LABELS[env] ?? env.toUpperCase();
}

// Flux carries no tenant id, so we derive it (and the environment) from the
// cluster id, which is `<tenant>_<environment>` (e.g. "acme_at23" -> tenant
// "acme", env "at23"). The environment is the segment after the last underscore;
// the tenant is everything before it (so multi-word tenants like
// "acme_apps_production" still resolve to tenant "acme_apps", env
// "production"). Mirrors dis-console's central.environmentOf / clusterID.

export function envOf(cluster: string): string {
  const i = cluster.lastIndexOf('_');
  return i >= 0 && i < cluster.length - 1 ? cluster.slice(i + 1) : '';
}

export function tenantOf(cluster: string): string {
  const i = cluster.lastIndexOf('_');
  return i > 0 ? cluster.slice(0, i) : cluster;
}

/** Flux kinds that represent a deployable "app" (the matrix rows). */
export const DEPLOYABLE_KINDS = ['Kustomization', 'HelmRelease'] as const;
/** Flux source kinds — plumbing (where to pull from), not apps. */
export const SOURCE_KINDS = ['OCIRepository', 'HelmRepository', 'HelmChart'] as const;

export function isDeployableKind(kind: string): boolean {
  return (DEPLOYABLE_KINDS as readonly string[]).includes(kind);
}

/** Workload kinds the agent sweeps (schema v5) — declared images live here. */
export const WORKLOAD_KINDS = ['Deployment', 'StatefulSet', 'DaemonSet'] as const;

export function isWorkloadKind(kind: string): boolean {
  return (WORKLOAD_KINDS as readonly string[]).includes(kind);
}

/**
 * What counts as an **app**: every HelmRelease, and every Kustomization that
 * is not an azapi root. The root Kustomization/OCIRepository pairs the azapi
 * fluxConfigurations create carry no kustomize owner labels (`appliedBy` is
 * null) — they are the plumbing that pulls the syncroot. HelmReleases are
 * never roots, so they qualify unconditionally (an unowned one is an anomaly
 * worth showing, not hiding). A workload applied directly by a root (no Flux
 * app of its own) also acts as an app; that needs the row context, so
 * `buildMatrix` decides it, not this predicate.
 */
export function isApp(r: Resource): boolean {
  if (r.kind === 'HelmRelease') return true;
  return r.kind === 'Kustomization' && Boolean(r.appliedBy);
}

/** DIS platform CRDs (`*.dis.altinn.cloud`) — provisioned Azure resources. */
export const DIS_KINDS = [
  'DatabaseServer',
  'Database',
  'Vault',
  'ApplicationIdentity',
  'Api',
  'ApiVersion',
  'Backend',
] as const;

export function isDisKind(kind: string): boolean {
  return (DIS_KINDS as readonly string[]).includes(kind);
}

/** DIS kinds grouped by the Azure product they provision — one console view
 *  per family. (Databases nests Database under DatabaseServer; APIM nests
 *  ApiVersion under Api.) */
export const DIS_PRODUCTS = {
  databases: ['DatabaseServer', 'Database'],
  identities: ['ApplicationIdentity'],
  apim: ['Api', 'ApiVersion', 'Backend'],
  vaults: ['Vault'],
} as const;

export type DisProduct = keyof typeof DIS_PRODUCTS;

export type DeployStatus =
  | 'healthy'
  | 'reconciling'
  | 'failed'
  | 'suspended'
  | 'unknown'
  | 'absent';

/** Worst-first severity of a status — folding and sorting share this order. */
export const STATUS_SEVERITY: Record<DeployStatus, number> = {
  failed: 5,
  suspended: 4,
  reconciling: 3,
  unknown: 2,
  healthy: 1,
  absent: 0,
};

/** Map a resource's Flux Ready state to a coarse deployment status. */
export function statusOf(r: Resource | undefined): DeployStatus {
  if (!r) return 'absent';
  if (r.suspended) return 'suspended';
  switch (r.ready) {
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

/**
 * Compact a Flux revision for display. Handles git shas
 * ("sha256:abcd…", "main@sha1:abcd…", "abcdef0123…") and Helm chart
 * revisions ("chart@1.2.3", "1.2.3").
 */
export function shortRev(rev: string | undefined): string {
  if (!rev) return '';
  const sha = rev.match(/sha\d*:([0-9a-f]+)/i);
  if (sha) return sha[1].slice(0, 7);
  if (rev.includes('@')) return rev.split('@').pop() ?? rev;
  if (/^[0-9a-f]{12,}$/i.test(rev)) return rev.slice(0, 7);
  return rev;
}
