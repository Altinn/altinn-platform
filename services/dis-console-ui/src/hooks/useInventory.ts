import { useEffect, useState } from 'react';
import { api } from '../api';
import type { InventoryResponse } from '../api/types';

export interface KustomizationRef {
  cluster: string;
  namespace: string;
  name: string;
}

/** Lazily fetches one Kustomization's applied-object inventory. */
export function useInventory(ref: KustomizationRef | null) {
  const [inventory, setInventory] = useState<InventoryResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { cluster, namespace, name } = ref ?? {};

  useEffect(() => {
    if (!cluster || !namespace || !name) {
      setInventory(null);
      setError(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    setInventory(null);
    api
      .getInventory(cluster, namespace, name)
      .then((inv) => {
        if (!cancelled) setInventory(inv);
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
  }, [cluster, namespace, name]);

  return { inventory, loading, error };
}
