import type { ArtifactFilters, FleetApi, ResourceFilters } from './client';
import type {
  Artifact,
  ArtifactsResponse,
  Cluster,
  ClustersResponse,
  InventoryResponse,
  Resource,
  ResourceDetail,
  ResourcesResponse,
  Summary,
} from './types';

const BASE = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`, { headers: { Accept: 'application/json' } });
  if (!res.ok) {
    throw new Error(`GET ${path} failed: ${res.status} ${res.statusText}`);
  }
  return (await res.json()) as T;
}

function queryString(filters: Record<string, string | undefined>): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) {
    if (value) params.set(key, value);
  }
  const s = params.toString();
  return s ? `?${s}` : '';
}

/** Live client against a deployed dis-console `server`. */
export class HttpFleetApi implements FleetApi {
  async getClusters(): Promise<Cluster[]> {
    return (await getJSON<ClustersResponse>('/api/clusters')).clusters;
  }

  async getSummary(cluster?: string): Promise<Summary> {
    const q = cluster ? `?cluster=${encodeURIComponent(cluster)}` : '';
    return getJSON<Summary>(`/api/summary${q}`);
  }

  async getResources(filters: ResourceFilters = {}): Promise<Resource[]> {
    return (await getJSON<ResourcesResponse>(`/api/resources${queryString({ ...filters })}`)).resources;
  }

  async getResource(
    cluster: string,
    kind: string,
    namespace: string,
    name: string,
  ): Promise<ResourceDetail> {
    const enc = encodeURIComponent;
    return getJSON<ResourceDetail>(
      `/api/resources/${enc(cluster)}/${enc(kind)}/${enc(namespace)}/${enc(name)}`,
    );
  }

  async getArtifacts(filters: ArtifactFilters = {}): Promise<Artifact[]> {
    return (await getJSON<ArtifactsResponse>(`/api/artifacts${queryString({ ...filters })}`)).artifacts;
  }

  async getInventory(cluster: string, namespace: string, name: string): Promise<InventoryResponse> {
    const enc = encodeURIComponent;
    return getJSON<InventoryResponse>(
      `/api/kustomizations/${enc(cluster)}/${enc(namespace)}/${enc(name)}/inventory`,
    );
  }
}
