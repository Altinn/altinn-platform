import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Resource } from '../api/types';

export interface ResourceRef {
  cluster: string;
  kind: string;
  namespace: string;
  name: string;
}

/** Lazily fetches one resource (including its raw object) for the detail drawer. */
export function useResourceDetail(ref: ResourceRef | null) {
  const [resource, setResource] = useState<Resource | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { cluster, kind, namespace, name } = ref ?? {};

  useEffect(() => {
    if (!cluster || !kind || !namespace || !name) {
      setResource(null);
      setError(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    setResource(null);
    api
      .getResource(cluster, kind, namespace, name)
      .then((r) => {
        if (!cancelled) setResource(r);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [cluster, kind, namespace, name]);

  return { resource, loading, error };
}
