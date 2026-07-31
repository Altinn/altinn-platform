import type { Resource, StatusEvent } from '../api/types';
import { shortRev, statusOf } from './flux';
import type { StageState } from './releases';

export interface AppRelease {
  revision: string;
  shortRev: string;
  /** Earliest time this revision was observed anywhere (from status history). */
  firstSeen?: string;
  chips: Record<string, StageState>;
}

/**
 * Derive an app's releases from its per-environment
 * status history (the detail endpoint's revision transitions): every distinct
 * revision is a release row, newest first; each env chip says whether that env
 * runs it now, ran it earlier, failed on it, or never had it.
 */
export function buildAppReleases(
  current: Partial<Record<string, Resource>>,
  histories: Partial<Record<string, StatusEvent[]>>,
  envs: string[],
): AppRelease[] {
  const firstSeen = new Map<string, string>();
  const revisions = new Set<string>();

  for (const env of envs) {
    const cur = current[env];
    if (cur?.revision) revisions.add(cur.revision);
    for (const ev of histories[env] ?? []) {
      if (!ev.revision) continue;
      revisions.add(ev.revision);
      const seen = firstSeen.get(ev.revision);
      if (!seen || ev.observedAt < seen) firstSeen.set(ev.revision, ev.observedAt);
    }
  }

  const rows: AppRelease[] = [...revisions].map((revision) => {
    const chips: Record<string, StageState> = {};
    for (const env of envs) {
      const cur = current[env];
      if (cur?.revision === revision) {
        const s = statusOf(cur);
        chips[env] =
          s === 'failed'
            ? 'failed'
            : s === 'reconciling'
              ? 'rolling'
              : s === 'suspended'
                ? 'suspended'
                : 'current';
        continue;
      }
      const past = (histories[env] ?? []).filter((ev) => ev.revision === revision);
      if (past.length > 0) {
        chips[env] = past.some((ev) => ev.ready === 'False') ? 'failed' : 'deployed';
      } else {
        chips[env] = 'absent';
      }
    }
    return { revision, shortRev: shortRev(revision), firstSeen: firstSeen.get(revision), chips };
  });

  // Newest first by first observation; revisions with no history timestamp
  // (only visible as a current value) sort first — they are the freshest.
  return rows.sort((a, b) => {
    if (!a.firstSeen && !b.firstSeen) return 0;
    if (!a.firstSeen) return -1;
    if (!b.firstSeen) return 1;
    return b.firstSeen.localeCompare(a.firstSeen);
  });
}

interface Revisioned {
  revision?: string;
}

/**
 * The owning Kustomization's revision that declared one release of a
 * HelmRelease. A chart version carries no commit of its own (packaging severs
 * the git lineage), but the chart-version bump lives in the gitops repo the
 * owner applies — so the owner's revision at that moment names the commit.
 * An environment currently running the release uses the owner's current
 * revision; otherwise the owner's status history is correlated by time (the
 * owner revision in effect when the release was first observed).
 */
export function ownerRevisionFor(
  revision: string,
  envs: string[],
  current: Partial<Record<string, Revisioned>>,
  owners: Partial<Record<string, Revisioned>>,
  histories: Partial<Record<string, StatusEvent[]>>,
  ownerHistories: Partial<Record<string, StatusEvent[]>>,
): string | undefined {
  for (const env of envs) {
    if (current[env]?.revision === revision && owners[env]?.revision) {
      return owners[env].revision;
    }
  }
  for (const env of envs) {
    const seen = (histories[env] ?? [])
      .filter((e) => e.revision === revision)
      .map((e) => e.observedAt)
      .sort()[0];
    if (!seen) continue;
    const owner = (ownerHistories[env] ?? [])
      .filter((o) => o.revision && o.observedAt <= seen)
      .sort((a, b) => b.observedAt.localeCompare(a.observedAt))[0];
    if (owner?.revision) return owner.revision;
  }
  return undefined;
}
