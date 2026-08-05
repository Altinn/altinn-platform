import type {
  Artifact,
  Cluster,
  InventoryResponse,
  Resource,
  ResourceDetail,
  Summary,
} from './types';

export interface ResourceFilters {
  cluster?: string;
  kind?: string;
  namespace?: string;
  ready?: string;
}

export interface ArtifactFilters {
  cluster?: string;
  class?: string;
}

/**
 * The read surface of the dis-console fleet API. Both the live HTTP client and
 * the bundled mock implement this; the UI only ever depends on the interface.
 */
export interface FleetApi {
  getClusters(): Promise<Cluster[]>;
  getSummary(cluster?: string): Promise<Summary>;
  getResources(filters?: ResourceFilters): Promise<Resource[]>;
  getResource(
    cluster: string,
    kind: string,
    namespace: string,
    name: string,
  ): Promise<ResourceDetail>;
  getArtifacts(filters?: ArtifactFilters): Promise<Artifact[]>;
  getInventory(cluster: string, namespace: string, name: string): Promise<InventoryResponse>;
}
