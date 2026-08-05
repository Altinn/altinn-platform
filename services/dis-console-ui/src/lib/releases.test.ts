import { describe, expect, it } from 'vitest';
import type { Artifact } from '../api/types';
import { buildArtifactMatrix } from './artifacts';
import { buildSyncrootReleases } from './releases';

function art(cluster: string, digest: string, applied: string, ready: 'True' | 'False' = 'True'): Artifact {
  return {
    cluster,
    namespace: 'flux-system',
    name: 'acme-syncroot',
    url: 'oci://reg.example/acme/syncroot',
    class: 'product-syncroot',
    owner: 'acme',
    revision: `tag@sha256:${digest}`,
    originSource: 'https://github.com/acme/gitops-manifests',
    originRevision: `main/${digest}0000000000000000000000000000000000`,
    ready,
    suspended: false,
    kustomizations: [
      { name: 'apps', namespace: 'flux-system', revision: `tag@sha256:${applied}`, ready, suspended: false },
    ],
  };
}

describe('buildSyncrootReleases', () => {
  it('derives release rows with current/rolling/absent chips, newest first', () => {
    const matrix = buildArtifactMatrix([
      art('acme_at22', 'aaa1111', 'aaa1111'),
      art('acme_at23', 'aaa1111', 'bbb2222'), // pulling the new release
      art('acme_production', 'bbb2222', 'bbb2222'),
    ]);
    const releases = buildSyncrootReleases(matrix.rows[0], matrix.envs);

    expect(releases.map((r) => r.digest)).toEqual(['aaa1111', 'bbb2222']);
    const [next, prev] = releases;
    expect(next.chips.at22).toBe('current');
    expect(next.chips.at23).toBe('rolling');
    expect(next.chips.production).toBe('absent');
    expect(prev.chips.at23).toBe('current'); // still applied while the new one rolls
    expect(prev.chips.production).toBe('current');
    expect(next.originSource).toContain('github.com');
  });

  it('marks a failed environment on the applied release', () => {
    const matrix = buildArtifactMatrix([art('acme_production', 'ccc3333', 'ccc3333', 'False')]);
    const [rel] = buildSyncrootReleases(matrix.rows[0], matrix.envs);
    expect(rel.chips.production).toBe('failed');
  });
});
