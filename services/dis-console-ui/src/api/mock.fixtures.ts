import { envOf } from '../lib/flux';
import type {
  Artifact,
  Cluster,
  ContainerImage,
  InventoryEntry,
  ReadyState,
  Resource,
  StatusEvent,
} from './types';

// Demo fleet data that mirrors the real /api/* JSON shapes. Names are fictional
// (placeholder tenants acme/fabrikam, generic app names) — not real services.
// It exercises every cell state the matrix can render: an app healthy across all
// stages, one failing in production, one reconciling, one mid-promotion (absent
// from later stages), one suspended — across two tenants.

const API_VERSION: Record<string, string> = {
  Kustomization: 'kustomize.toolkit.fluxcd.io/v1',
  HelmRelease: 'helm.toolkit.fluxcd.io/v2',
  OCIRepository: 'source.toolkit.fluxcd.io/v1',
  Deployment: 'apps/v1',
};

const ORIGIN_REPO = 'https://github.com/acme/gitops-manifests';

interface EnvState {
  ready?: ReadyState;
  rev?: string;
  suspended?: boolean;
  reason?: string;
  message?: string;
  ageHours?: number;
  /** Declared container images — for workload kinds (schema v5). */
  images?: ContainerImage[];
  /** Per-env origin revision override (sources whose digest differs per env). */
  originRevision?: string;
}

interface AppSpec {
  tenant: string;
  kind: keyof typeof API_VERSION;
  namespace: string;
  name: string;
  /** The owning Kustomization (HelmReleases applied by an app Kustomization). */
  appliedBy?: { name: string; namespace: string };
  /** A Kustomization's Flux source — the join to its repo for commit links. */
  sourceRef?: { kind: string; name: string; namespace?: string };
  /** A source's origin git repo (the OCI annotation the fleet API projects). */
  originSource?: string;
  /** A source's origin git revision annotation, `branch/sha`. */
  originRevision?: string;
  envs: Partial<Record<string, EnvState>>;
}

function iso(hoursAgo: number): string {
  return new Date(Date.now() - hoursAgo * 3_600_000).toISOString();
}

const SPECS: AppSpec[] = [
  {
    // Newer revision rolled out through AT/TT, still reconciling on TT02,
    // older revision on YT01/Production -> revision drift, mid-promotion feel.
    tenant: 'acme',
    kind: 'Kustomization',
    namespace: 'acme',
    name: 'frontend',
    appliedBy: { name: 'acme-apps', namespace: 'flux-system' },
    sourceRef: { kind: 'OCIRepository', name: 'acme-syncroot', namespace: 'flux-system' },
    envs: {
      at22: { rev: 'main@sha1:9f3c1a2', ageHours: 5 },
      at23: { rev: 'main@sha1:9f3c1a2', ageHours: 5 },
      at24: { rev: 'main@sha1:9f3c1a2', ageHours: 4 },
      tt02: {
        ready: 'Unknown',
        rev: 'main@sha1:9f3c1a2',
        reason: 'Progressing',
        message: 'Reconciliation in progress: applying manifests',
        ageHours: 0.2,
      },
      yt01: { rev: 'main@sha1:4b7e0d8', ageHours: 72 },
      production: { rev: 'main@sha1:4b7e0d8', ageHours: 72 },
    },
  },
  {
    // The app Kustomization that applies the api-gateway chart below — the
    // matrix folds the chart's health into this row.
    tenant: 'acme',
    kind: 'Kustomization',
    namespace: 'acme',
    name: 'api-gateway',
    appliedBy: { name: 'acme-apps', namespace: 'flux-system' },
    sourceRef: { kind: 'OCIRepository', name: 'acme-syncroot', namespace: 'flux-system' },
    envs: {
      at22: { rev: 'main@sha1:8a2b4c6', ageHours: 30 },
      at23: { rev: 'main@sha1:8a2b4c6', ageHours: 30 },
      at24: { rev: 'main@sha1:8a2b4c6', ageHours: 30 },
      tt02: { rev: 'main@sha1:8a2b4c6', ageHours: 26 },
      yt01: { rev: 'main@sha1:8a2b4c6', ageHours: 26 },
      production: { rev: 'main@sha1:8a2b4c6', ageHours: 26 },
    },
  },
  {
    // Healthy everywhere except a failed Helm install in production — folded
    // into the api-gateway Kustomization row via appliedBy.
    tenant: 'acme',
    kind: 'HelmRelease',
    namespace: 'acme',
    name: 'api-gateway',
    appliedBy: { name: 'api-gateway', namespace: 'acme' },
    envs: {
      at22: { rev: '1.4.7', ageHours: 30 },
      at23: { rev: '1.4.7', ageHours: 30 },
      at24: { rev: '1.4.7', ageHours: 30 },
      tt02: { rev: '1.4.7', ageHours: 26 },
      yt01: { rev: '1.4.7', ageHours: 26 },
      production: {
        ready: 'False',
        rev: '1.4.6',
        reason: 'InstallFailed',
        message: 'Helm install failed: timed out waiting for the condition',
        ageHours: 1,
      },
    },
  },
  {
    // Only promoted to the AT stages so far.
    tenant: 'acme',
    kind: 'Kustomization',
    namespace: 'acme-platform',
    name: 'worker',
    appliedBy: { name: 'acme-apps', namespace: 'flux-system' },
    sourceRef: { kind: 'OCIRepository', name: 'acme-syncroot', namespace: 'flux-system' },
    envs: {
      at22: { rev: 'main@sha1:c0ffee1', ageHours: 8 },
      at23: { rev: 'main@sha1:c0ffee1', ageHours: 8 },
      at24: { rev: 'main@sha1:c0ffee1', ageHours: 7 },
    },
  },
  {
    // Suspended on AT24 (reconciliation paused), healthy elsewhere.
    tenant: 'acme',
    kind: 'Kustomization',
    namespace: 'acme-platform',
    name: 'scheduler',
    appliedBy: { name: 'acme-apps', namespace: 'flux-system' },
    sourceRef: { kind: 'OCIRepository', name: 'acme-syncroot', namespace: 'flux-system' },
    envs: {
      at22: { rev: '2.0.1', ageHours: 50 },
      at23: { rev: '2.0.1', ageHours: 50 },
      at24: { rev: '2.0.1', suspended: true, reason: 'Suspended', ageHours: 120 },
      tt02: { rev: '2.0.1', ageHours: 48 },
      yt01: { rev: '2.0.1', ageHours: 48 },
      production: { rev: '2.0.1', ageHours: 48 },
    },
  },
  {
    tenant: 'fabrikam',
    kind: 'Kustomization',
    namespace: 'fabrikam',
    name: 'web',
    appliedBy: { name: 'fabrikam-apps', namespace: 'flux-system' },
    envs: {
      at22: { rev: 'main@sha1:d15ea5e', ageHours: 12 },
      at23: { rev: 'main@sha1:d15ea5e', ageHours: 12 },
      at24: { rev: 'main@sha1:d15ea5e', ageHours: 11 },
      tt02: { rev: 'main@sha1:d15ea5e', ageHours: 10 },
    },
  },
  {
    tenant: 'fabrikam',
    kind: 'HelmRelease',
    namespace: 'fabrikam-platform',
    name: 'payments',
    appliedBy: { name: 'fabrikam-apps', namespace: 'flux-system' },
    envs: {
      at22: {
        ready: 'Unknown',
        rev: '0.9.3',
        reason: 'Progressing',
        message: 'Helm upgrade in progress',
        ageHours: 0.1,
      },
      at23: { rev: '0.9.3', ageHours: 6 },
      at24: { rev: '0.9.3', ageHours: 6 },
    },
  },
  {
    // The frontend app's Deployment (kustomize-applied, so the v5 agent sweeps
    // it): the image tag tells the promotion story — 1.42.0 through the AT/TT
    // stages, 1.41.2 still in YT/Production (drift), a sidecar that never
    // drifts, and TT02 mid-rollout.
    tenant: 'acme',
    kind: 'Deployment',
    namespace: 'acme',
    name: 'frontend',
    appliedBy: { name: 'frontend', namespace: 'acme' },
    envs: {
      at22: {
        images: [
          { container: 'frontend', image: 'ghcr.io/acme/frontend:1.42.0' },
          { container: 'otel-agent', image: 'ghcr.io/acme/otel-agent:0.8.1' },
        ],
        ageHours: 5,
      },
      at23: {
        images: [
          { container: 'frontend', image: 'ghcr.io/acme/frontend:1.42.0' },
          { container: 'otel-agent', image: 'ghcr.io/acme/otel-agent:0.8.1' },
        ],
        ageHours: 5,
      },
      at24: {
        images: [
          { container: 'frontend', image: 'ghcr.io/acme/frontend:1.42.0' },
          { container: 'otel-agent', image: 'ghcr.io/acme/otel-agent:0.8.1' },
        ],
        ageHours: 4,
      },
      tt02: {
        ready: 'Unknown',
        reason: 'ReadyReplicas',
        message: '2/3 ready',
        images: [
          { container: 'frontend', image: 'ghcr.io/acme/frontend:1.42.0' },
          { container: 'otel-agent', image: 'ghcr.io/acme/otel-agent:0.8.1' },
        ],
        ageHours: 0.2,
      },
      yt01: {
        images: [
          { container: 'frontend', image: 'ghcr.io/acme/frontend:1.41.2' },
          { container: 'otel-agent', image: 'ghcr.io/acme/otel-agent:0.8.1' },
        ],
        ageHours: 72,
      },
      production: {
        images: [
          { container: 'frontend', image: 'ghcr.io/acme/frontend:1.41.2' },
          { container: 'otel-agent', image: 'ghcr.io/acme/otel-agent:0.8.1' },
        ],
        ageHours: 72,
      },
    },
  },
  {
    tenant: 'acme',
    kind: 'Deployment',
    namespace: 'acme-platform',
    name: 'worker',
    appliedBy: { name: 'worker', namespace: 'acme-platform' },
    envs: {
      at22: {
        images: [{ container: 'worker', image: 'ghcr.io/acme/worker:0.9.1' }],
        ageHours: 8,
      },
      at23: {
        images: [{ container: 'worker', image: 'ghcr.io/acme/worker:0.9.1' }],
        ageHours: 8,
      },
      at24: {
        images: [{ container: 'worker', image: 'ghcr.io/acme/worker:0.9.1' }],
        ageHours: 7,
      },
    },
  },
  {
    // Applied DIRECTLY by the root Kustomization — no app of its own (the
    // dis-console-on-admin pattern). Visible in the syncroot Workloads tab
    // via the closure, deliberately absent from the Releases app list.
    tenant: 'acme',
    kind: 'Deployment',
    namespace: 'acme-platform',
    name: 'metrics-agent',
    appliedBy: { name: 'acme-apps', namespace: 'flux-system' },
    envs: {
      at22: {
        images: [{ container: 'agent', image: 'ghcr.io/acme/metrics-agent:3.1.0' }],
        ageHours: 20,
      },
      at23: {
        images: [{ container: 'agent', image: 'ghcr.io/acme/metrics-agent:3.1.0' }],
        ageHours: 20,
      },
      production: {
        images: [{ container: 'agent', image: 'ghcr.io/acme/metrics-agent:3.0.2' }],
        ageHours: 90,
      },
    },
  },
  {
    // A chart-created workload carrying Helm ownership (the future fleet-API
    // projection for HelmRelease apps): appliedBy names the HelmRelease.
    tenant: 'fabrikam',
    kind: 'Deployment',
    namespace: 'fabrikam-platform',
    name: 'payments',
    appliedBy: { name: 'payments', namespace: 'fabrikam-platform' },
    envs: {
      at22: {
        images: [{ container: 'payments', image: 'ghcr.io/fabrikam/payments:0.9.3' }],
        ageHours: 6,
      },
      at23: {
        images: [{ container: 'payments', image: 'ghcr.io/fabrikam/payments:0.9.3' }],
        ageHours: 6,
      },
      at24: {
        images: [{ container: 'payments', image: 'ghcr.io/fabrikam/payments:0.9.3' }],
        ageHours: 6,
      },
    },
  },
  {
    tenant: 'fabrikam',
    kind: 'Deployment',
    namespace: 'fabrikam',
    name: 'web',
    appliedBy: { name: 'web', namespace: 'fabrikam' },
    envs: {
      at22: {
        images: [{ container: 'web', image: 'ghcr.io/fabrikam/web:2.3.4' }],
        ageHours: 12,
      },
      at23: {
        images: [{ container: 'web', image: 'ghcr.io/fabrikam/web:2.3.4' }],
        ageHours: 12,
      },
      at24: {
        images: [{ container: 'web', image: 'ghcr.io/fabrikam/web:2.3.4' }],
        ageHours: 11,
      },
      tt02: {
        images: [{ container: 'web', image: 'ghcr.io/fabrikam/web:2.3.4' }],
        ageHours: 10,
      },
    },
  },
  {
    // The syncroot ROOT Kustomizations, as the azapi fluxConfiguration creates
    // them (AKS GitOps extension — no kustomize owner labels, so appliedBy is
    // null). They are plumbing, not apps: the apps views must exclude them,
    // while the explicit Kind filter still shows them. Revisions mirror the
    // acme-syncroot artifact fixture (at23 mid-rollout on the older digest).
    tenant: 'acme',
    kind: 'Kustomization',
    namespace: 'flux-system',
    name: 'acme-apps',
    sourceRef: { kind: 'OCIRepository', name: 'acme-syncroot', namespace: 'flux-system' },
    envs: {
      at22: { rev: 'at22@sha256:9f3c1a2e4b6d', ageHours: 5 },
      at23: {
        ready: 'Unknown',
        rev: 'at23@sha256:4b7e0d8c1f3a',
        reason: 'Progressing',
        message: 'Reconciliation in progress: applying revision',
        ageHours: 0.3,
      },
      at24: { rev: 'at24@sha256:9f3c1a2e4b6d', ageHours: 4 },
      tt02: { rev: 'tt02@sha256:9f3c1a2e4b6d', ageHours: 4 },
      yt01: { rev: 'yt01@sha256:4b7e0d8c1f3a', ageHours: 72 },
      production: { rev: 'production@sha256:4b7e0d8c1f3a', ageHours: 72 },
    },
  },
  {
    tenant: 'fabrikam',
    kind: 'Kustomization',
    namespace: 'flux-system',
    name: 'fabrikam-apps',
    sourceRef: { kind: 'OCIRepository', name: 'fabrikam-syncroot', namespace: 'flux-system' },
    envs: {
      at22: { rev: 'at22@sha256:d15ea5e77c2b', ageHours: 12 },
      at23: { rev: 'at23@sha256:d15ea5e77c2b', ageHours: 12 },
      at24: { rev: 'at24@sha256:d15ea5e77c2b', ageHours: 11 },
      tt02: { rev: 'tt02@sha256:d15ea5e77c2b', ageHours: 10 },
    },
  },
  {
    tenant: 'fabrikam',
    kind: 'OCIRepository',
    namespace: 'flux-system',
    name: 'fabrikam-syncroot',
    originSource: 'https://github.com/fabrikam/gitops-manifests',
    originRevision: 'main/d15ea5e77c2b4f8a9b0c1d2e3f40516273849506',
    envs: {
      at22: { rev: 'at22@sha256:d15ea5e77c2b', ageHours: 12 },
      at23: { rev: 'at23@sha256:d15ea5e77c2b', ageHours: 12 },
      at24: { rev: 'at24@sha256:d15ea5e77c2b', ageHours: 11 },
      tt02: { rev: 'tt02@sha256:d15ea5e77c2b', ageHours: 10 },
    },
  },
  {
    // A source (OCIRepository) — excluded from the matrix by default; visible via
    // the Kind filter. Real fleets have a syncroot source per cluster.
    tenant: 'acme',
    kind: 'OCIRepository',
    namespace: 'flux-system',
    name: 'acme-syncroot',
    originSource: ORIGIN_REPO,
    envs: {
      at22: {
        rev: 'at22@sha256:9f3c1a2e4b6d',
        originRevision: 'main/9f3c1a2e4b6d0000000000000000000000000000',
        ageHours: 5,
      },
      at23: {
        rev: 'at23@sha256:9f3c1a2e4b6d',
        originRevision: 'main/9f3c1a2e4b6d0000000000000000000000000000',
        ageHours: 5,
      },
      at24: {
        rev: 'at24@sha256:9f3c1a2e4b6d',
        originRevision: 'main/9f3c1a2e4b6d0000000000000000000000000000',
        ageHours: 4,
      },
      tt02: {
        rev: 'tt02@sha256:9f3c1a2e4b6d',
        originRevision: 'main/9f3c1a2e4b6d0000000000000000000000000000',
        ageHours: 4,
      },
      yt01: {
        rev: 'yt01@sha256:4b7e0d8c1f3a',
        originRevision: 'main/4b7e0d8c1f3a0000000000000000000000000000',
        ageHours: 72,
      },
      production: {
        rev: 'production@sha256:4b7e0d8c1f3a',
        originRevision: 'main/4b7e0d8c1f3a0000000000000000000000000000',
        ageHours: 72,
      },
    },
  },
];

function buildResources(specs: AppSpec[]): Resource[] {
  const out: Resource[] = [];
  for (const spec of specs) {
    for (const [env, state] of Object.entries(spec.envs)) {
      if (!state) continue;
      const ready: ReadyState = state.ready ?? 'True';
      out.push({
        kind: spec.kind,
        apiVersion: API_VERSION[spec.kind],
        namespace: spec.namespace,
        name: spec.name,
        ready,
        reason: state.reason ?? (ready === 'True' ? 'ReconciliationSucceeded' : undefined),
        message: state.message,
        revision: state.rev,
        suspended: state.suspended ?? false,
        generation: 1,
        observedGeneration: 1,
        lastTransition: iso(state.ageHours ?? 6),
        ...(spec.appliedBy ? { appliedBy: spec.appliedBy } : {}),
        ...(spec.sourceRef ? { sourceRef: spec.sourceRef } : {}),
        ...(spec.originSource ? { originSource: spec.originSource } : {}),
        ...((state.originRevision ?? spec.originRevision)
          ? { originRevision: state.originRevision ?? spec.originRevision }
          : {}),
        ...(state.images ? { images: state.images } : {}),
        cluster: `${spec.tenant}_${env}`,
      });
    }
  }
  return out;
}

// What each demo revision superseded — lets the mock detail endpoint serve a
// plausible status-event history (the releases view is built from it).
const PREV_REV: Record<string, string> = {
  'main@sha1:9f3c1a2': 'main@sha1:4b7e0d8',
  'main@sha1:4b7e0d8': 'main@sha1:77aa310',
  'main@sha1:8a2b4c6': 'main@sha1:7f1e9d0',
  '1.4.7': '1.4.6',
  '1.4.6': '1.4.5',
  'main@sha1:c0ffee1': 'main@sha1:b00c0de',
  '2.0.1': '2.0.0',
  'main@sha1:d15ea5e': 'main@sha1:c4a9b21',
  '0.9.3': '0.9.2',
};

function hoursAgoOf(t?: string): number {
  if (!t) return 6;
  return Math.max(0, (Date.now() - Date.parse(t)) / 3_600_000);
}

/** Status-event history for the detail endpoint (newest first): the current
 *  state plus, for deployables, up to two superseded revisions ~3 days apart. */
export function historyFor(r: Resource): StatusEvent[] {
  const out: StatusEvent[] = [
    {
      ready: r.ready,
      reason: r.reason,
      revision: r.revision,
      observedAt: r.lastTransition ?? iso(6),
    },
  ];
  if ((r.kind === 'Kustomization' || r.kind === 'HelmRelease') && r.revision) {
    let rev: string | undefined = PREV_REV[r.revision];
    let hoursBack = hoursAgoOf(r.lastTransition) + 68;
    for (let n = 0; rev && n < 2; n++) {
      out.push({
        ready: 'True',
        reason: 'ReconciliationSucceeded',
        revision: rev,
        observedAt: iso(hoursBack),
      });
      rev = PREV_REV[rev];
      hoursBack += 68;
    }
  }
  return out;
}

// DIS platform resources — the DatabaseServer + its Databases, Vaults,
// managed identities, and APIM APIs a team's namespace owns. Fictional;
// `azureResourceId` is the ARM id the DIS operators surface in status, used to
// build Portal links.
// (Databases are logical — no direct ARM id — so they nest under their server.)
const SUB = '00000000-0000-0000-0000-000000000000';
const DIS_API: Record<string, string> = {
  DatabaseServer: 'storage.dis.altinn.cloud/v1alpha1',
  Database: 'storage.dis.altinn.cloud/v1alpha1',
  Vault: 'vault.dis.altinn.cloud/v1alpha1',
  ApplicationIdentity: 'application.dis.altinn.cloud/v1alpha1',
  Api: 'apim.dis.altinn.cloud/v1alpha1',
  ApiVersion: 'apim.dis.altinn.cloud/v1alpha1',
  Backend: 'apim.dis.altinn.cloud/v1alpha1',
};
const PG = 'Microsoft.DBforPostgreSQL/flexibleServers';
const KV = 'Microsoft.KeyVault/vaults';
const MI = 'Microsoft.ManagedIdentity/userAssignedIdentities';
const APIM = 'Microsoft.ApiManagement/service';

function armId(rg: string, typePath: string, name: string): string {
  return `/subscriptions/${SUB}/resourceGroups/${rg}/providers/${typePath}/${name}`;
}

function dis(
  cluster: string,
  namespace: string,
  kind: keyof typeof DIS_API,
  name: string,
  extra: Partial<Resource> = {},
): Resource {
  return {
    kind,
    apiVersion: DIS_API[kind],
    namespace,
    name,
    ready: 'True',
    reason: 'Succeeded',
    suspended: false,
    generation: 1,
    observedGeneration: 1,
    lastTransition: iso(24),
    cluster,
    ...extra,
  };
}

const APPLIED_BY_FRONTEND = { appliedBy: { name: 'frontend', namespace: 'acme' } };
const APPLIED_BY_GATEWAY = { appliedBy: { name: 'api-gateway', namespace: 'acme' } };
const APPLIED_BY_WEB = { appliedBy: { name: 'web', namespace: 'fabrikam' } };

const DIS_RESOURCES: Resource[] = [
  dis('acme_at23', 'acme', 'DatabaseServer', 'acme-db', { azureResourceId: armId('acme_at23', PG, 'acme-db'), ...APPLIED_BY_FRONTEND }),
  dis('acme_at23', 'acme', 'Database', 'acme-app', { parent: { kind: 'DatabaseServer', name: 'acme-db' }, ...APPLIED_BY_FRONTEND }),
  dis('acme_at23', 'acme', 'Database', 'acme-jobs', {
    parent: { kind: 'DatabaseServer', name: 'acme-db' },
    ready: 'Unknown',
    reason: 'Progressing',
    message: 'Reconciliation in progress: creating database',
    ...APPLIED_BY_FRONTEND,
  }),
  dis('acme_at23', 'acme', 'Vault', 'acme-kv', { azureResourceId: armId('acme_at23', KV, 'acme-kv'), ...APPLIED_BY_FRONTEND }),
  dis('acme_at23', 'acme', 'ApplicationIdentity', 'acme-app-id', { azureResourceId: armId('acme_at23', MI, 'acme-app-id'), ...APPLIED_BY_FRONTEND }),
  dis('acme_at23', 'acme-platform', 'Vault', 'acme-platform-kv', {
    azureResourceId: armId('acme_at23', KV, 'acme-platform-kv'),
    appliedBy: { name: 'worker', namespace: 'acme-platform' },
  }),
  dis('acme_at23', 'acme-platform', 'ApplicationIdentity', 'platform-agent-id', {
    azureResourceId: armId('acme_at23', MI, 'platform-agent-id'),
    appliedBy: { name: 'worker', namespace: 'acme-platform' },
  }),
  dis('acme_at23', 'acme', 'Api', 'orders-api', { azureResourceId: armId('acme_at23', `${APIM}/acme-apim/apiVersionSets`, 'orders-api'), ...APPLIED_BY_GATEWAY }),
  dis('acme_at23', 'acme', 'ApiVersion', 'orders-api-v1', { parent: { kind: 'Api', name: 'orders-api' }, ...APPLIED_BY_GATEWAY }),
  dis('acme_at23', 'acme', 'ApiVersion', 'orders-api-v2', {
    parent: { kind: 'Api', name: 'orders-api' },
    ready: 'Unknown',
    reason: 'Progressing',
    message: 'Reconciliation in progress: importing OpenAPI spec',
    ...APPLIED_BY_GATEWAY,
  }),
  dis('acme_at23', 'acme', 'Backend', 'orders-backend', { azureResourceId: armId('acme_at23', `${APIM}/acme-apim/backends`, 'orders-backend'), ...APPLIED_BY_GATEWAY }),

  dis('acme_production', 'acme', 'DatabaseServer', 'acme-db', { azureResourceId: armId('acme_production', PG, 'acme-db'), ...APPLIED_BY_FRONTEND }),
  dis('acme_production', 'acme', 'Database', 'acme-app', { parent: { kind: 'DatabaseServer', name: 'acme-db' }, ...APPLIED_BY_FRONTEND }),
  dis('acme_production', 'acme', 'Vault', 'acme-kv', { azureResourceId: armId('acme_production', KV, 'acme-kv'), ...APPLIED_BY_FRONTEND }),

  dis('fabrikam_at23', 'fabrikam', 'DatabaseServer', 'fabrikam-db', { azureResourceId: armId('fabrikam_at23', PG, 'fabrikam-db'), ...APPLIED_BY_WEB }),
  dis('fabrikam_at23', 'fabrikam', 'Database', 'fabrikam-web', { parent: { kind: 'DatabaseServer', name: 'fabrikam-db' }, ...APPLIED_BY_WEB }),
  dis('fabrikam_at23', 'fabrikam', 'Vault', 'fabrikam-kv', { azureResourceId: armId('fabrikam_at23', KV, 'fabrikam-kv'), ...APPLIED_BY_WEB }),
];

export const RESOURCES: Resource[] = [...buildResources(SPECS), ...DIS_RESOURCES];

function buildClusters(resources: Resource[]): Cluster[] {
  const counts = new Map<string, number>();
  for (const r of resources) counts.set(r.cluster, (counts.get(r.cluster) ?? 0) + 1);
  return [...counts.keys()].sort().map((cluster) => {
    // Make one cluster look stale to exercise the staleness banner.
    const stale = cluster === 'acme_yt01';
    return {
      cluster,
      environment: envOf(cluster),
      lastSweepAt: iso(stale ? 50 : 0.05),
      lastSyncedAt: iso(stale ? 50 : 0.03),
      agentVersion: '1.1.0',
      schemaVersion: 2,
      resourceCount: counts.get(cluster) ?? 0,
      stale,
    };
  });
}

export const CLUSTERS: Cluster[] = buildClusters(RESOURCES);

// Base-layer OCI artifacts — the platform layer under the apps: product
// syncroots, infra packages, and operator configs, as /api/artifacts serves
// them. Fictional registry + digests; the digest is the artifact's version
// (tags are mutable environment/ring tags).
const REG = 'oci://registry.example.io';

interface ArtifactSpec {
  tenant: string;
  namespace: string;
  name: string;
  url: string;
  class: string;
  owner: string;
  kustomization: { name: string; namespace: string };
  /** env → { digest, applied (defaults to digest), ready }. */
  envs: Partial<Record<string, { digest: string; applied?: string; ready?: ReadyState; tag?: string }>>;
}

const ARTIFACT_SPECS: ArtifactSpec[] = [
  {
    tenant: 'acme',
    namespace: 'flux-system',
    name: 'acme-syncroot',
    url: `${REG}/acme/syncroot`,
    class: 'product-syncroot',
    owner: 'acme',
    kustomization: { name: 'acme-apps', namespace: 'flux-system' },
    envs: {
      at22: { digest: '9f3c1a2e4b6d' },
      at23: { digest: '9f3c1a2e4b6d', applied: '4b7e0d8c1f3a' },
      at24: { digest: '9f3c1a2e4b6d' },
      tt02: { digest: '9f3c1a2e4b6d' },
      yt01: { digest: '4b7e0d8c1f3a' },
      production: { digest: '4b7e0d8c1f3a' },
    },
  },
  {
    tenant: 'fabrikam',
    namespace: 'flux-system',
    name: 'fabrikam-syncroot',
    url: `${REG}/fabrikam/syncroot`,
    class: 'product-syncroot',
    owner: 'fabrikam',
    kustomization: { name: 'fabrikam-apps', namespace: 'flux-system' },
    envs: {
      at22: { digest: 'd15ea5e77c2b' },
      at23: { digest: 'd15ea5e77c2b' },
      at24: { digest: 'd15ea5e77c2b' },
      tt02: { digest: 'd15ea5e77c2b' },
    },
  },
  {
    tenant: 'acme',
    namespace: 'monitoring',
    name: 'monitoring',
    url: `${REG}/manifests/infra/monitoring`,
    class: 'infra',
    owner: 'monitoring',
    kustomization: { name: 'monitoring', namespace: 'monitoring' },
    envs: {
      at22: { digest: 'c0ffee1a2b3c' },
      at23: { digest: 'c0ffee1a2b3c' },
      production: { digest: 'c0ffee1a2b3c', ready: 'False' },
    },
  },
  {
    tenant: 'acme',
    namespace: 'flux-system',
    name: 'dis-pgsql-operator',
    url: `${REG}/dis/kustomize/dis-pgsql-operator`,
    class: 'operator',
    owner: 'dis-pgsql-operator',
    kustomization: { name: 'dis-pgsql-operator', namespace: 'flux-system' },
    envs: {
      at22: { digest: 'f04438b12e89', tag: 'at_ring1' },
      at23: { digest: 'f04438b12e89', tag: 'at_ring1' },
      at24: { digest: 'f04438b12e89', tag: 'at_ring1' },
      tt02: { digest: 'e93327a01d78', tag: 'tt_ring' },
      yt01: { digest: 'e93327a01d78', tag: 'tt_ring' },
      production: { digest: 'e93327a01d78', tag: 'prod_ring' },
    },
  },
];

function buildArtifacts(specs: ArtifactSpec[]): Artifact[] {
  const out: Artifact[] = [];
  for (const spec of specs) {
    for (const [env, state] of Object.entries(spec.envs)) {
      if (!state) continue;
      const tag = state.tag ?? env;
      const revision = `${tag}@sha256:${state.digest}`;
      const applied = `${tag}@sha256:${state.applied ?? state.digest}`;
      out.push({
        cluster: `${spec.tenant}_${env}`,
        namespace: spec.namespace,
        name: spec.name,
        url: spec.url,
        class: spec.class,
        owner: spec.owner,
        revision,
        originRevision: `main/${state.digest}${'0'.repeat(40 - state.digest.length)}`,
        originSource: ORIGIN_REPO,
        ready: state.ready ?? 'True',
        suspended: false,
        kustomizations: [
          {
            name: spec.kustomization.name,
            namespace: spec.kustomization.namespace,
            revision: applied,
            ready: state.applied ? 'Unknown' : (state.ready ?? 'True'),
            suspended: false,
          },
        ],
      });
    }
  }
  return out;
}

export const ARTIFACTS: Artifact[] = buildArtifacts(ARTIFACT_SPECS);

// Inventory (a Kustomization's applied-object set) for the syncroot
// Kustomizations: the app objects above plus a few unswept kinds (null
// resource), mirroring /api/kustomizations/{...}/inventory.
const INVENTORY_SPECS: Record<string, { group?: string; kind: string; namespace?: string; name: string }[]> = {
  'acme_at23|flux-system|acme-apps': [
    { group: 'kustomize.toolkit.fluxcd.io', kind: 'Kustomization', namespace: 'acme', name: 'frontend' },
    { group: 'kustomize.toolkit.fluxcd.io', kind: 'Kustomization', namespace: 'acme', name: 'api-gateway' },
    { group: 'kustomize.toolkit.fluxcd.io', kind: 'Kustomization', namespace: 'acme-platform', name: 'worker' },
    { kind: 'Namespace', name: 'acme' },
    { kind: 'ServiceAccount', namespace: 'acme', name: 'deployer' },
  ],
  'acme_at23|acme|frontend': [
    { group: 'apps', kind: 'Deployment', namespace: 'acme', name: 'frontend' },
    { kind: 'Service', namespace: 'acme', name: 'frontend' },
    { kind: 'ConfigMap', namespace: 'acme', name: 'frontend-config' },
    { group: 'storage.dis.altinn.cloud', kind: 'DatabaseServer', namespace: 'acme', name: 'acme-db' },
    { group: 'vault.dis.altinn.cloud', kind: 'Vault', namespace: 'acme', name: 'acme-kv' },
    { group: 'application.dis.altinn.cloud', kind: 'ApplicationIdentity', namespace: 'acme', name: 'acme-app-id' },
  ],
  'acme_at23|acme|api-gateway': [
    { group: 'helm.toolkit.fluxcd.io', kind: 'HelmRelease', namespace: 'acme', name: 'api-gateway' },
    { kind: 'ServiceAccount', namespace: 'acme', name: 'api-gateway' },
  ],
  'acme_at23|acme-platform|worker': [
    { group: 'apps', kind: 'Deployment', namespace: 'acme-platform', name: 'worker' },
    { group: 'vault.dis.altinn.cloud', kind: 'Vault', namespace: 'acme-platform', name: 'acme-platform-kv' },
  ],
};

export function inventoryFor(cluster: string, namespace: string, name: string): InventoryEntry[] {
  const specs = INVENTORY_SPECS[`${cluster}|${namespace}|${name}`] ?? [];
  return specs.map((s) => ({
    ...s,
    version: 'v1',
    resource:
      RESOURCES.find(
        (r) =>
          r.cluster === cluster &&
          r.kind === s.kind &&
          r.namespace === (s.namespace ?? r.namespace) &&
          r.name === s.name,
      ) ?? null,
  }));
}

/** Synthesize a plausible raw Flux object for the detail drawer. */
export function rawFor(r: Resource): unknown {
  return {
    apiVersion: r.apiVersion,
    kind: r.kind,
    metadata: {
      name: r.name,
      namespace: r.namespace,
      generation: r.generation,
      // HelmReleases are applied by a Kustomization — Flux records the owner in
      // these labels. Root Kustomizations (no appliedBy) are provisioned via
      // azapi fluxConfigurations, which stamp Azure's clusterconfig labels.
      ...(r.kind === 'HelmRelease'
        ? {
            labels: {
              'kustomize.toolkit.fluxcd.io/name': r.appliedBy?.name ?? r.name,
              'kustomize.toolkit.fluxcd.io/namespace': r.appliedBy?.namespace ?? 'flux-system',
            },
          }
        : {}),
      ...(r.kind === 'Kustomization' && !r.appliedBy
        ? {
            labels: {
              'clusterconfig.azure.com/name': r.name,
              'clusterconfig.azure.com/namespace': r.namespace,
              'clusterconfig.azure.com/is-managed': 'true',
            },
          }
        : {}),
    },
    spec: {
      interval: '5m',
      ...(r.suspended ? { suspend: true } : {}),
    },
    status: {
      observedGeneration: r.observedGeneration,
      lastAppliedRevision: r.revision,
      // Fictional source annotations so the drawer's "View source" link is
      // demoable on mock data. Real Flux records these on the source object
      // (OCIRepository), not on each Kustomization/HelmRelease.
      artifact: {
        metadata: {
          'org.opencontainers.image.source': 'https://github.com/acme/gitops-manifests',
          'org.opencontainers.image.revision': 'main/0a1b2c3d4e5f60718293a4b5c6d7e8f901234567',
        },
      },
      conditions: [
        {
          type: 'Ready',
          status: r.ready,
          reason: r.reason,
          message: r.message ?? (r.ready === 'True' ? `Applied revision: ${r.revision ?? ''}` : ''),
          lastTransitionTime: r.lastTransition,
        },
      ],
    },
  };
}
