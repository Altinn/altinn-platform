import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Artifact } from '../api/types';

interface ArtifactsState {
  artifacts: Artifact[];
  loading: boolean;
  error: string | null;
}

/** Loads the fleet's base-layer artifacts once. */
export function useArtifacts() {
  const [state, setState] = useState<ArtifactsState>({ artifacts: [], loading: true, error: null });

  useEffect(() => {
    let cancelled = false;
    api
      .getArtifacts()
      .then((artifacts) => {
        if (!cancelled) setState({ artifacts, loading: false, error: null });
      })
      .catch((e: unknown) => {
        if (!cancelled)
          setState({ artifacts: [], loading: false, error: e instanceof Error ? e.message : String(e) });
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return state;
}
