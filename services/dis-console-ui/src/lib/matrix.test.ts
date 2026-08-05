import { describe, expect, it } from 'vitest';
import type { Resource } from '../api/types';
import { buildMatrix, kindsOf, namespacesOf, tenantsOf, worstOf } from './matrix';

function res(partial: Partial<Resource>): Resource {
  return {
    kind: 'Kustomization',
    apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
    namespace: 'acme',
    name: 'app',
    ready: 'True',
    suspended: false,
    cluster: 'acme_at23',
    // Created by us: shipped through a syncroot, so the kustomize owner
    // labels are present. Roots (azapi fluxConfigurations) override to null.
    appliedBy: { name: 'acme-root', namespace: 'flux-system' },
    ...partial,
  };
}

describe('buildMatrix', () => {
  it('groups one resource per environment into a single row', () => {
    const m = buildMatrix([
      res({ name: 'web', cluster: 'acme_at22', revision: 'r1' }),
      res({ name: 'web', cluster: 'acme_at23', revision: 'r1' }),
      res({ name: 'web', cluster: 'acme_production', revision: 'r1' }),
    ]);
    expect(m.rows).toHaveLength(1);
    expect(m.rows[0].name).toBe('web');
    expect(m.rows[0].cells.at22?.status).toBe('healthy');
    expect(m.rows[0].cells.production?.status).toBe('healthy');
    expect(m.rows[0].cells.tt02).toBeUndefined();
  });

  it('renders only environments the tenant has clusters in, in promotion order', () => {
    const m = buildMatrix([
      res({ name: 'a', cluster: 'acme_production' }),
      res({ name: 'b', cluster: 'acme_at22' }),
    ]);
    expect(m.envs).toEqual(['at22', 'production']);
  });

  it('keeps a column the app is absent from when the env exists tenant-wide', () => {
    const m = buildMatrix(
      [
        res({ name: 'web', cluster: 'acme_at22' }),
        res({ name: 'other', namespace: 'elsewhere', cluster: 'acme_production' }),
      ],
      { tenant: 'acme', namespace: 'acme' },
    );
    expect(m.envs).toEqual(['at22', 'production']);
    expect(m.rows.map((r) => r.name)).toEqual(['web']);
  });

  it('gives a tenant with its own ring its own columns (admin test/prod)', () => {
    const m = buildMatrix(
      [res({ cluster: 'admin_prod' }), res({ cluster: 'admin_test' })],
      { tenant: 'admin' },
    );
    expect(m.envs).toEqual(['test', 'prod']);
  });

  it('flags revision drift across environments', () => {
    const m = buildMatrix([
      res({ name: 'web', cluster: 'acme_at23', revision: 'main@sha1:newnewn' }),
      res({ name: 'web', cluster: 'acme_production', revision: 'main@sha1:oldoldo' }),
    ]);
    expect(m.rows[0].drift).toBe(true);
  });

  it('does not flag drift when revisions agree', () => {
    const m = buildMatrix([
      res({ name: 'web', cluster: 'acme_at23', revision: 'r1' }),
      res({ name: 'web', cluster: 'acme_production', revision: 'r1' }),
    ]);
    expect(m.rows[0].drift).toBe(false);
  });

  it('sorts rows with failures first', () => {
    const m = buildMatrix([
      res({ name: 'healthy-app', cluster: 'acme_at23', ready: 'True' }),
      res({ name: 'broken-app', cluster: 'acme_production', ready: 'False' }),
    ]);
    expect(m.rows.map((r) => r.name)).toEqual(['broken-app', 'healthy-app']);
    expect(m.rows[0].anyFailed).toBe(true);
  });

  it('scopes rows to a tenant', () => {
    const all = [
      res({ name: 'a', namespace: 'acme', cluster: 'acme_at23' }),
      res({ name: 'b', namespace: 'fabrikam', cluster: 'fabrikam_at23' }),
    ];
    const m = buildMatrix(all, { tenant: 'acme' });
    expect(m.rows.map((r) => r.name)).toEqual(['a']);
  });

  it('scopes rows to a namespace', () => {
    const all = [
      res({ name: 'a', namespace: 'ns1', cluster: 'acme_at23' }),
      res({ name: 'b', namespace: 'ns2', cluster: 'acme_at23' }),
    ];
    const m = buildMatrix(all, { namespace: 'ns2' });
    expect(m.rows.map((r) => r.name)).toEqual(['b']);
  });

  it('records a conflict when two clusters map to the same env slot', () => {
    const m = buildMatrix([
      res({ name: 'web', cluster: 'acme_at23', revision: 'r1' }),
      res({ name: 'web', cluster: 'acme_at23', revision: 'r2' }),
    ]);
    expect(m.rows[0].cells.at23?.conflict).toHaveLength(2);
  });
});

describe('tenantsOf / namespacesOf', () => {
  const data = [
    res({ namespace: 'ns1', cluster: 'acme_at23' }),
    res({ namespace: 'ns2', cluster: 'acme_tt02' }),
    res({ namespace: 'ns3', cluster: 'fabrikam_at23' }),
  ];

  it('lists every tenant, sorted', () => {
    expect(tenantsOf(data)).toEqual(['acme', 'fabrikam']);
  });

  it('lists namespaces, optionally scoped to a tenant', () => {
    expect(namespacesOf(data)).toEqual(['ns1', 'ns2', 'ns3']);
    expect(namespacesOf(data, 'acme')).toEqual(['ns1', 'ns2']);
  });
});

describe('kind filtering', () => {
  const data = [
    res({ kind: 'Kustomization', name: 'app', cluster: 'acme_at23' }),
    res({ kind: 'HelmRelease', name: 'chart', cluster: 'acme_at23' }),
    res({ kind: 'OCIRepository', name: 'src', namespace: 'flux-system', cluster: 'acme_at23' }),
  ];

  it('excludes source kinds by default (apps only)', () => {
    const m = buildMatrix(data);
    expect(m.rows.map((r) => r.name).sort()).toEqual(['app', 'chart']);
  });

  it('keeps an unowned HelmRelease as an app (only azapi roots are plumbing)', () => {
    const m = buildMatrix([
      res({ kind: 'HelmRelease', name: 'orphan', appliedBy: undefined, ready: 'False' }),
    ]);
    expect(m.rows.map((r) => r.name)).toEqual(['orphan']);
  });

  it('folds a chart workload into an unowned HelmRelease instead of promoting it', () => {
    const m = buildMatrix([
      res({ kind: 'HelmRelease', name: 'orphan', appliedBy: undefined }),
      res({
        kind: 'Deployment',
        apiVersion: 'apps/v1',
        name: 'orphan-manager',
        appliedBy: { name: 'orphan', namespace: 'acme' },
      }),
    ]);
    expect(m.rows).toHaveLength(1);
    expect(m.rows[0].kind).toBe('HelmRelease');
    expect(m.rows[0].cells.at23?.children).toHaveLength(1);
  });

  it('excludes azapi fluxConfiguration roots (no appliedBy) from the apps view', () => {
    const m = buildMatrix([
      ...data,
      res({ name: 'acme-4f2a-acme', namespace: 'product-acme', appliedBy: undefined }),
    ]);
    expect(m.rows.map((r) => r.name).sort()).toEqual(['app', 'chart']);
  });

  it('shows azapi roots flat under an explicit kind filter', () => {
    const m = buildMatrix(
      [
        res({ name: 'app' }),
        res({ name: 'acme-4f2a-acme', namespace: 'product-acme', appliedBy: undefined }),
      ],
      { kind: 'Kustomization' },
    );
    expect(m.rows.map((r) => r.name).sort()).toEqual(['acme-4f2a-acme', 'app']);
  });

  it('filters to a single kind when given', () => {
    const m = buildMatrix(data, { kind: 'OCIRepository' });
    expect(m.rows.map((r) => r.name)).toEqual(['src']);
  });

  it('kindsOf lists the kinds present', () => {
    expect(kindsOf(data)).toEqual(['HelmRelease', 'Kustomization', 'OCIRepository']);
  });
});

describe('workloads as apps', () => {
  const dep = (partial: Partial<Resource>) =>
    res({ kind: 'Deployment', apiVersion: 'apps/v1', ...partial });

  it('a workload applied by a root becomes its own row', () => {
    const m = buildMatrix([
      dep({
        name: 'dis-console',
        namespace: 'product-dis',
        appliedBy: { name: 'admin-root', namespace: 'flux-system' },
      }),
    ]);
    expect(m.rows.map((r) => `${r.kind}/${r.name}`)).toEqual(['Deployment/dis-console']);
  });

  it('a workload applied by an app Kustomization folds into it, worst-of status', () => {
    const m = buildMatrix([
      res({ name: 'frontend' }),
      dep({ name: 'frontend', ready: 'False', appliedBy: { name: 'frontend', namespace: 'acme' } }),
    ]);
    expect(m.rows).toHaveLength(1);
    expect(m.rows[0].kind).toBe('Kustomization');
    expect(m.rows[0].cells.at23?.status).toBe('failed');
  });

  it('a chart workload folds into its standalone HelmRelease row', () => {
    const m = buildMatrix([
      res({ kind: 'HelmRelease', name: 'payments', namespace: 'pay' }),
      dep({ name: 'payments-web', namespace: 'pay', appliedBy: { name: 'payments', namespace: 'pay' } }),
    ]);
    expect(m.rows).toHaveLength(1);
    expect(m.rows[0].kind).toBe('HelmRelease');
    expect(m.rows[0].cells.at23?.children).toHaveLength(1);
  });

  it('a workload without appliedBy stays out of the apps view', () => {
    const m = buildMatrix([dep({ name: 'stray', appliedBy: undefined })]);
    expect(m.rows).toHaveLength(0);
  });
});

describe('appliedBy folding', () => {
  it('folds an owned HelmRelease into its Kustomization row, worst-of status', () => {
    const m = buildMatrix([
      res({ name: 'shop', cluster: 'acme_production' }),
      res({
        kind: 'HelmRelease',
        name: 'shop-chart',
        cluster: 'acme_production',
        ready: 'False',
        appliedBy: { name: 'shop', namespace: 'acme' },
      }),
    ]);
    expect(m.rows).toHaveLength(1);
    expect(m.rows[0].kind).toBe('Kustomization');
    expect(m.rows[0].cells.production?.status).toBe('failed');
    expect(m.rows[0].cells.production?.children?.map((c) => c.name)).toEqual(['shop-chart']);
    expect(m.rows[0].anyFailed).toBe(true);
  });

  it('folds across namespaces via the appliedBy namespace', () => {
    const m = buildMatrix([
      res({ name: 'grafana-operator', namespace: 'flux-system', cluster: 'acme_at23' }),
      res({
        kind: 'HelmRelease',
        name: 'grafana-operator',
        namespace: 'grafana',
        cluster: 'acme_at23',
        appliedBy: { name: 'grafana-operator', namespace: 'flux-system' },
      }),
    ]);
    expect(m.rows).toHaveLength(1);
    expect(m.rows[0].namespace).toBe('flux-system');
    expect(m.rows[0].cells.at23?.children).toHaveLength(1);
  });

  it('keeps a HelmRelease with no in-data owner as its own row', () => {
    const m = buildMatrix([
      res({
        kind: 'HelmRelease',
        name: 'standalone',
        cluster: 'acme_at23',
        appliedBy: { name: 'ghost', namespace: 'flux-system' },
      }),
    ]);
    expect(m.rows).toHaveLength(1);
    expect(m.rows[0].kind).toBe('HelmRelease');
  });

  it('shows HelmReleases flat under an explicit kind filter', () => {
    const m = buildMatrix(
      [
        res({ name: 'shop', cluster: 'acme_at23' }),
        res({
          kind: 'HelmRelease',
          name: 'shop-chart',
          cluster: 'acme_at23',
          appliedBy: { name: 'shop', namespace: 'acme' },
        }),
      ],
      { kind: 'HelmRelease' },
    );
    expect(m.rows.map((r) => r.name)).toEqual(['shop-chart']);
  });

  it('folds into a synthesized cell when the owner is absent from that env', () => {
    const m = buildMatrix([
      res({ name: 'shop', cluster: 'acme_at22' }),
      res({
        kind: 'HelmRelease',
        name: 'shop-chart',
        cluster: 'acme_at23',
        ready: 'Unknown',
        appliedBy: { name: 'shop', namespace: 'acme' },
      }),
    ]);
    expect(m.rows).toHaveLength(1);
    expect(m.rows[0].cells.at23?.status).toBe('reconciling');
    expect(m.rows[0].cells.at23?.resource).toBeUndefined();
  });

  it('ranks folded statuses worst-first', () => {
    expect(worstOf('healthy', 'failed')).toBe('failed');
    expect(worstOf('failed', 'healthy')).toBe('failed');
    expect(worstOf('healthy', 'reconciling')).toBe('reconciling');
    expect(worstOf('reconciling', 'suspended')).toBe('suspended');
    expect(worstOf('absent', 'healthy')).toBe('healthy');
  });
});
