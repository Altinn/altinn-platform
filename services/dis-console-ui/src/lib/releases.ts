import type { ArtifactRow } from './artifacts';
import { artifactStatus, shortDigest } from './artifacts';

/** How one release relates to one environment (a stage chip). */
export type StageState =
  | 'current'
  | 'deployed' // ran there earlier, since superseded
  | 'rolling'
  | 'failed'
  | 'suspended'
  | 'unknown'
  | 'absent';

export interface ReleaseRow {
  /** The release identity — the artifact digest (short). */
  digest: string;
  /** Origin annotations of an environment fetching this digest, when known. */
  originSource?: string;
  originRevision?: string;
  chips: Record<string, StageState>;
}

/**
 * Derive a syncroot's releases: every distinct digest seen
 * across the environments (fetched or applied) is a release row; each env chip
 * says whether that env currently runs it (`current`), is pulling it
 * (`rolling`), or doesn't have it (`absent`). Rows are ordered newest-first
 * using ring semantics: the digest reaching the earliest environment in the
 * promotion order is the newest.
 */
export function buildSyncrootReleases(row: ArtifactRow, envs: string[]): ReleaseRow[] {
  const rows = new Map<string, ReleaseRow>();
  const firstSeen = new Map<string, number>();

  const touch = (digest: string, envIdx: number) => {
    if (!digest) return;
    if (!rows.has(digest)) rows.set(digest, { digest, chips: {} });
    firstSeen.set(digest, Math.min(firstSeen.get(digest) ?? Infinity, envIdx));
  };

  envs.forEach((env, i) => {
    const cell = row.cells[env];
    if (!cell) return;
    const fetched = shortDigest(cell.artifact.revision);
    touch(fetched, i);
    for (const k of cell.artifact.kustomizations) {
      touch(shortDigest(k.revision), i);
    }
  });

  for (const release of rows.values()) {
    for (let i = 0; i < envs.length; i++) {
      const env = envs[i];
      const cell = row.cells[env];
      if (!cell) {
        release.chips[env] = 'absent';
        continue;
      }
      const fetched = shortDigest(cell.artifact.revision);
      const applied = cell.artifact.kustomizations.some(
        (k) => shortDigest(k.revision) === release.digest,
      );
      if (applied) {
        const status = artifactStatus(cell.artifact);
        release.chips[env] =
          status === 'failed' ? 'failed' : status === 'suspended' ? 'suspended' : 'current';
      } else if (fetched === release.digest) {
        release.chips[env] = 'rolling';
      } else {
        release.chips[env] = 'absent';
      }
      if (fetched === release.digest && !release.originSource) {
        release.originSource = cell.artifact.originSource;
        release.originRevision = cell.artifact.originRevision;
      }
    }
  }

  return [...rows.values()].sort(
    (a, b) => (firstSeen.get(a.digest) ?? Infinity) - (firstSeen.get(b.digest) ?? Infinity),
  );
}
