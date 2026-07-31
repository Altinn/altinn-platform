import { describe, expect, it } from 'vitest';
import type { Resource, StatusEvent } from '../api/types';
import { buildAppReleases, ownerRevisionFor } from './appReleases';

const res = (revision: string, ready: 'True' | 'False' | 'Unknown' = 'True'): Resource => ({
  kind: 'Kustomization',
  apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
  namespace: 'acme',
  name: 'frontend',
  ready,
  suspended: false,
  revision,
  cluster: 'acme_at23',
});

const ev = (revision: string, observedAt: string, ready: 'True' | 'False' = 'True'): StatusEvent => ({
  ready,
  revision,
  observedAt,
});

describe('buildAppReleases', () => {
  it('builds release rows from history, newest first, with per-env chips', () => {
    const envs = ['at22', 'production'];
    const rows = buildAppReleases(
      { at22: res('r2'), production: res('r1') },
      {
        at22: [ev('r2', '2026-07-19T10:00:00Z'), ev('r1', '2026-07-10T10:00:00Z')],
        production: [ev('r1', '2026-07-11T10:00:00Z')],
      },
      envs,
    );
    expect(rows.map((r) => r.revision)).toEqual(['r2', 'r1']);
    expect(rows[0].chips).toEqual({ at22: 'current', production: 'absent' });
    expect(rows[1].chips).toEqual({ at22: 'deployed', production: 'current' });
    expect(rows[1].firstSeen).toBe('2026-07-10T10:00:00Z');
  });

  it('marks failed history and rolling current state', () => {
    const rows = buildAppReleases(
      { at22: res('r2', 'Unknown') },
      { at22: [ev('r1', '2026-07-01T00:00:00Z', 'False')] },
      ['at22'],
    );
    const r2 = rows.find((r) => r.revision === 'r2');
    const r1 = rows.find((r) => r.revision === 'r1');
    expect(r2?.chips.at22).toBe('rolling');
    expect(r1?.chips.at22).toBe('failed');
  });
});

describe('ownerRevisionFor', () => {
  const envs = ['at22', 'production'];

  it('uses the owner current revision for an env running the release', () => {
    const rev = ownerRevisionFor(
      '2.19.0',
      envs,
      { at22: { revision: '2.19.0' } },
      { at22: { revision: 'main@sha1:abc1234' } },
      {},
      {},
    );
    expect(rev).toBe('main@sha1:abc1234');
  });

  it('correlates a historical release with the owner revision in effect', () => {
    const rev = ownerRevisionFor(
      '2.18.0',
      envs,
      { at22: { revision: '2.19.0' } },
      { at22: { revision: 'main@sha1:ccc3333' } },
      { at22: [ev('2.18.0', '2026-07-10T10:00:00Z')] },
      {
        at22: [
          ev('main@sha1:bbb2222', '2026-07-09T10:00:00Z'),
          ev('main@sha1:ccc3333', '2026-07-11T10:00:00Z'),
        ],
      },
    );
    expect(rev).toBe('main@sha1:bbb2222');
  });

  it('gives nothing when no owner revision predates the release', () => {
    const rev = ownerRevisionFor(
      '2.17.0',
      envs,
      {},
      {},
      { at22: [ev('2.17.0', '2026-07-01T10:00:00Z')] },
      { at22: [ev('main@sha1:ddd4444', '2026-07-05T10:00:00Z')] },
    );
    expect(rev).toBeUndefined();
  });
});
