import { useEffect, useMemo, useState } from 'react';
import { api } from '../api';
import type { ResourceDetail, StatusEvent } from '../api/types';
import { isWorkloadKind } from '../lib/flux';
import type { MatrixRow } from '../lib/matrix';

interface EnvDetail {
  history: StatusEvent[];
  /** For HelmRelease and workload rows: the owning Kustomization's detail —
   *  its revision (and history) name the gitops commits that declared them. */
  owner?: ResourceDetail;
}

interface State {
  key: string;
  details: Partial<Record<string, EnvDetail>>;
}

/**
 * Load one app's status-event history for every environment it exists in
 * (one detail fetch per env, in parallel; HelmRelease rows also fetch their
 * owning Kustomization). An env whose fetch fails — or a server old enough to
 * not serve `history` — degrades to an empty history, so the releases view
 * still renders from the current state.
 */
export function useAppHistories(row: MatrixRow) {
  const [state, setState] = useState<State | null>(null);

  useEffect(() => {
    let cancelled = false;
    const cells = Object.values(row.cells).filter((c) => c.resource);
    Promise.all(
      cells.map(async (c) => {
        const r = c.resource!;
        let history: StatusEvent[] = [];
        try {
          history = (await api.getResource(r.cluster, r.kind, r.namespace, r.name)).history ?? [];
        } catch {
          // keep the empty history
        }
        let owner: ResourceDetail | undefined;
        if ((r.kind === 'HelmRelease' || isWorkloadKind(r.kind)) && r.appliedBy) {
          try {
            owner = await api.getResource(
              r.cluster,
              'Kustomization',
              r.appliedBy.namespace,
              r.appliedBy.name,
            );
          } catch {
            // no owner mirrored on this cluster
          }
        }
        return [c.env, { history, owner }] as const;
      }),
    ).then((entries) => {
      if (!cancelled) setState({ key: row.key, details: Object.fromEntries(entries) });
    });
    return () => {
      cancelled = true;
    };
  }, [row]);

  const ready = state?.key === row.key;
  const details = ready ? state.details : undefined;
  return useMemo(() => {
    const histories: Partial<Record<string, StatusEvent[]>> = {};
    const owners: Partial<Record<string, ResourceDetail>> = {};
    const ownerHistories: Partial<Record<string, StatusEvent[]>> = {};
    for (const [env, d] of Object.entries(details ?? {})) {
      if (!d) continue;
      histories[env] = d.history;
      if (d.owner) {
        owners[env] = d.owner;
        ownerHistories[env] = d.owner.history ?? [];
      }
    }
    return {
      histories: details ? histories : undefined,
      owners,
      ownerHistories,
      loading: !details,
    };
  }, [details]);
}
