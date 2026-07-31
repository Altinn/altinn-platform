import { describe, expect, it } from 'vitest';
import type { Resource } from '../api/types';
import { buildDisResources, disEnvsOf, disTenantsOf } from './disResources';

function res(partial: Partial<Resource>): Resource {
  return {
    kind: 'Vault',
    apiVersion: 'vault.dis.altinn.cloud/v1alpha1',
    namespace: 'acme',
    name: 'x',
    ready: 'True',
    suspended: false,
    cluster: 'acme_at23',
    ...partial,
  };
}

describe('buildDisResources', () => {
  const data = [
    res({ kind: 'DatabaseServer', name: 'acme-db' }),
    res({ kind: 'Database', name: 'app', parent: { kind: 'DatabaseServer', name: 'acme-db' } }),
    res({ kind: 'Database', name: 'jobs', parent: { kind: 'DatabaseServer', name: 'acme-db' } }),
    res({ kind: 'Vault', name: 'acme-kv' }),
    res({ kind: 'Kustomization', name: 'not-dis' }), // excluded (not a DIS kind)
    res({ kind: 'Vault', name: 'other', cluster: 'acme_production' }), // other cluster
  ];

  it('groups a cluster by namespace, nesting children under their parent', () => {
    const groups = buildDisResources(data, { cluster: 'acme_at23' });
    expect(groups).toHaveLength(1);
    expect(groups[0].namespace).toBe('acme');
    const server = groups[0].nodes.find((n) => n.resource.kind === 'DatabaseServer');
    expect(server?.children.map((c) => c.name)).toEqual(['app', 'jobs']);
    expect(groups[0].nodes.some((n) => n.resource.name === 'acme-kv')).toBe(true);
    expect(groups[0].nodes.some((n) => n.resource.name === 'not-dis')).toBe(false);
  });

  it('scopes to the given cluster', () => {
    const groups = buildDisResources(data, { cluster: 'acme_production' });
    expect(groups.flatMap((g) => g.nodes).map((n) => n.resource.name)).toEqual(['other']);
  });

  it('filters to the given product kinds', () => {
    const groups = buildDisResources(data, { cluster: 'acme_at23', kinds: ['Vault'] });
    expect(groups.flatMap((g) => g.nodes).map((n) => n.resource.name)).toEqual(['acme-kv']);
  });
});

describe('disTenantsOf / disEnvsOf', () => {
  const data = [
    res({ cluster: 'acme_at23' }),
    res({ cluster: 'acme_production' }),
    res({ kind: 'DatabaseServer', cluster: 'fabrikam_at23' }),
    res({ kind: 'Kustomization', cluster: 'zzz_at23' }), // not DIS -> ignored
  ];

  it('lists DIS tenants sorted', () => {
    expect(disTenantsOf(data)).toEqual(['acme', 'fabrikam']);
  });

  it('lists DIS envs for a tenant in promotion order', () => {
    expect(disEnvsOf(data, 'acme')).toEqual(['at23', 'production']);
  });
});
