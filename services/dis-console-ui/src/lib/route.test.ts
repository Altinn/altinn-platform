import { describe, expect, it } from 'vitest';
import { parseRoute, routeHash } from './route';

describe('routes', () => {
  it('defaults to home', () => {
    expect(parseRoute('')).toEqual({ view: 'home' });
    expect(parseRoute('#/')).toEqual({ view: 'home' });
    expect(parseRoute('#/nonsense')).toEqual({ view: 'home' });
  });

  it('round-trips section views', () => {
    for (const view of ['deployments', 'syncroots', 'databases', 'vaults'] as const) {
      expect(parseRoute(routeHash({ view }))).toEqual({ view });
    }
  });

  it('round-trips a syncroot page with env and a key containing separators', () => {
    const r = { view: 'syncroot', key: 'product-syncroot|acme|flux-system|acme-syncroot', env: 'at23' } as const;
    const hash = routeHash(r);
    expect(hash).toContain('%7C'); // the | separators survive encoding
    expect(parseRoute(hash)).toEqual(r);
  });

  it('round-trips a syncroot tab, and drops a tab that has no env to sit under', () => {
    const r = {
      view: 'syncroot',
      key: 'product-syncroot|acme|flux-system|acme-syncroot',
      env: 'at23',
      tab: 'releases',
    } as const;
    expect(parseRoute(routeHash(r))).toEqual(r);
    expect(routeHash({ view: 'syncroot', key: 'k', tab: 'releases' })).toBe('#/syncroots/k');
  });

  it('round-trips a kustomization page', () => {
    const r = { view: 'kustomization', cluster: 'acme_at23', namespace: 'flux-system', name: 'acme-apps' } as const;
    expect(parseRoute(routeHash(r))).toEqual(r);
  });
});
