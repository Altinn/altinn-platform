import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Resource } from '../api/types';
import { sourceLinkFromRaw, sourceRefFromRaw, type SourceLink } from '../lib/sourceLink';

/**
 * Resolves a "view source" link for a loaded resource. If the resource is itself
 * a source (OCIRepository/...) its raw carries the OCI annotations directly;
 * otherwise it has a spec.sourceRef, and we fetch that source's detail to read
 * the annotations. Returns null when no source link can be derived.
 */
export function useSourceLink(resource: Resource | null) {
  const [link, setLink] = useState<SourceLink | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setLink(null);
    setLoading(false);
    if (!resource?.raw) return;

    const direct = sourceLinkFromRaw(resource.raw);
    if (direct) {
      setLink(direct);
      return;
    }

    const ref = sourceRefFromRaw(resource.raw);
    if (!ref) return;

    let cancelled = false;
    setLoading(true);
    api
      .getResource(resource.cluster, ref.kind, ref.namespace ?? resource.namespace, ref.name)
      .then((src) => {
        if (!cancelled) setLink(sourceLinkFromRaw(src.raw));
      })
      .catch(() => {
        if (!cancelled) setLink(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [resource]);

  return { link, loading };
}
