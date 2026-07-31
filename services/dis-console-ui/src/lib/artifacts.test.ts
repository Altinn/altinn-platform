import { describe, expect, it } from 'vitest';
import type { Artifact, Resource } from '../api/types';
import {
  artifactClassesOf,
  artifactEnvsOf,
  artifactTag,
  buildArtifactMatrix,
  deployedBySyncroot,
  inFlight,
  shortDigest,
  syncrootsFor,
  syncrootSummaries,
} from './artifacts';

function art(partial: Partial<Artifact>): Artifact {
  return {
    cluster: 'acme_at23',
    namespace: 'flux-system',
    name: 'acme-syncroot',
    url: 'oci://registry.example.io/acme/syncroot',
    class: 'product-syncroot',
    owner: 'acme',
    revision: 'at23@sha256:9f3c1a2000000',
    ready: 'True',
    suspended: false,
    kustomizations: [],
    ...partial,
  };
}

describe('shortDigest / artifactTag', () => {
  it('extracts the digest from tag@sha256 revisions', () => {
    expect(shortDigest('at23@sha256:9f3c1a2bc4d5e6f7')).toBe('9f3c1a2');
    expect(artifactTag('at23@sha256:9f3c1a2bc4d5e6f7')).toBe('at23');
  });

  it('handles bare and missing revisions', () => {
    expect(shortDigest(undefined)).toBe('');
    expect(shortDigest('sha256:abcdef012345')).toBe('abcdef0');
    expect(artifactTag('v1.2.3')).toBe('');
  });
});

describe('inFlight', () => {
  it('is true when a kustomization lags the artifact digest', () => {
    const a = art({
      revision: 'at23@sha256:aaaaaaa1111',
      kustomizations: [
        { name: 'apps', namespace: 'flux-system', revision: 'at23@sha256:bbbbbbb2222', ready: 'True', suspended: false },
      ],
    });
    expect(inFlight(a)).toBe(true);
  });

  it('is false when applied digests match or are absent', () => {
    const a = art({
      revision: 'at23@sha256:aaaaaaa1111',
      kustomizations: [
        { name: 'apps', namespace: 'flux-system', revision: 'at23@sha256:aaaaaaa1111', ready: 'True', suspended: false },
        { name: 'new', namespace: 'flux-system', ready: 'Unknown', suspended: false },
      ],
    });
    expect(inFlight(a)).toBe(false);
  });
});

describe('buildArtifactMatrix', () => {
  const data = [
    art({ cluster: 'acme_at22', revision: 'at22@sha256:aaaaaaa' }),
    art({ cluster: 'acme_production', revision: 'prod@sha256:bbbbbbb' }),
    art({
      cluster: 'acme_at22',
      class: 'operator',
      owner: 'dis-pgsql-operator',
      name: 'dis-pgsql-operator',
      url: 'oci://registry.example.io/dis/kustomize/dis-pgsql-operator',
    }),
    art({ cluster: 'fabrikam_at22', owner: 'fabrikam', name: 'fabrikam-syncroot' }),
  ];

  it('groups one artifact identity per row across environments', () => {
    const m = buildArtifactMatrix(data, { tenant: 'acme' });
    expect(m.rows).toHaveLength(2);
    const syncroot = m.rows.find((r) => r.class === 'product-syncroot');
    expect(syncroot?.cells.at22?.artifact.revision).toBe('at22@sha256:aaaaaaa');
    expect(syncroot?.cells.production?.artifact.revision).toBe('prod@sha256:bbbbbbb');
  });

  it('sorts syncroots before operators and filters by class', () => {
    const m = buildArtifactMatrix(data, { tenant: 'acme' });
    expect(m.rows.map((r) => r.class)).toEqual(['product-syncroot', 'operator']);
    const only = buildArtifactMatrix(data, { tenant: 'acme', class: 'operator' });
    expect(only.rows).toHaveLength(1);
  });

  it('scopes tenants and lists classes present', () => {
    const m = buildArtifactMatrix(data, { tenant: 'fabrikam' });
    expect(m.rows).toHaveLength(1);
    expect(artifactClassesOf(data, 'acme')).toEqual(['product-syncroot', 'operator']);
  });

  it('lists environments per tenant in promotion order', () => {
    expect(artifactEnvsOf(data, 'acme')).toEqual(['at22', 'production']);
  });

  it('picks only syncroot-class artifacts of one cluster as tree roots', () => {
    const roots = syncrootsFor(data, 'acme_at22');
    expect(roots.map((r) => r.name)).toEqual(['acme-syncroot']);
    expect(roots[0].class).toBe('product-syncroot');
  });
});

describe('deployedBySyncroot', () => {
  const res = (partial: Partial<Resource>): Resource => ({
    kind: 'Kustomization',
    apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
    namespace: 'acme',
    name: 'x',
    ready: 'True',
    suspended: false,
    cluster: 'acme_at23',
    ...partial,
  });

  const data = [
    // app Kustomization applied by the root
    res({ name: 'frontend', appliedBy: { name: 'acme-apps', namespace: 'flux-system' } }),
    // CRs applied by the app Kustomization (transitive)
    res({ kind: 'DatabaseServer', name: 'acme-db', appliedBy: { name: 'frontend', namespace: 'acme' } }),
    res({ kind: 'HelmRelease', name: 'chart', appliedBy: { name: 'frontend', namespace: 'acme' } }),
    // unrelated: different owner chain
    res({ kind: 'Vault', name: 'other-kv', appliedBy: { name: 'other-root', namespace: 'flux-system' } }),
    // same chain, other cluster
    res({ name: 'frontend', cluster: 'acme_at22', appliedBy: { name: 'acme-apps', namespace: 'flux-system' } }),
    // root itself (no appliedBy) — never part of the result
    res({ name: 'acme-apps', namespace: 'flux-system' }),
  ];

  it('walks the appliedBy chain from the root Kustomizations', () => {
    const out = deployedBySyncroot(data, 'acme_at23', [{ name: 'acme-apps', namespace: 'flux-system' }]);
    expect(out.map((r) => `${r.kind}/${r.name}`)).toEqual([
      'DatabaseServer/acme-db',
      'HelmRelease/chart',
      'Kustomization/frontend',
    ]);
  });

  it('scopes to the cluster and the given roots', () => {
    const out = deployedBySyncroot(data, 'acme_at22', [{ name: 'acme-apps', namespace: 'flux-system' }]);
    expect(out.map((r) => r.name)).toEqual(['frontend']);
  });

  it('summarizes a syncroot like a project: namespaces, envs, worst status', () => {
    const arts = [
      art({
        cluster: 'acme_at23',
        revision: 'at23@sha256:aaa1111',
        kustomizations: [
          { name: 'acme-apps', namespace: 'flux-system', revision: 'at23@sha256:aaa1111', ready: 'True', suspended: false },
        ],
      }),
      art({
        cluster: 'acme_production',
        ready: 'False',
        revision: 'prod@sha256:bbb2222',
        kustomizations: [],
      }),
    ];
    const rs = [
      res({ name: 'frontend', namespace: 'acme', appliedBy: { name: 'acme-apps', namespace: 'flux-system' } }),
      res({ kind: 'Vault', name: 'kv', namespace: 'acme-platform', appliedBy: { name: 'frontend', namespace: 'acme' } }),
    ];
    const [s] = syncrootSummaries(arts, rs);
    expect(s.owner).toBe('acme');
    expect(s.envs).toEqual(['at23', 'production']);
    expect(s.namespaces).toEqual(['acme', 'acme-platform']);
    expect(s.worst).toBe('failed');
  });
});
