import { useCallback, useEffect, useState } from 'react';
import { api } from '../api';
import type { Cluster, Resource } from '../api/types';

interface FleetState {
  clusters: Cluster[];
  resources: Resource[];
  loading: boolean;
  error: string | null;
}

/** Loads the fleet's clusters and resources once, with a manual reload. */
export function useFleet() {
  const [state, setState] = useState<FleetState>({
    clusters: [],
    resources: [],
    loading: true,
    error: null,
  });

  const load = useCallback(async () => {
    setState((s) => ({ ...s, loading: true, error: null }));
    try {
      const [clusters, resources] = await Promise.all([api.getClusters(), api.getResources()]);
      setState({ clusters, resources, loading: false, error: null });
    } catch (e) {
      setState((s) => ({ ...s, loading: false, error: e instanceof Error ? e.message : String(e) }));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return { ...state, reload: load };
}
