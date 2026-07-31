import type { ArtifactFilters, FleetApi, ResourceFilters } from './client';
import type {
  Artifact,
  Cluster,
  InventoryResponse,
  KindSummary,
  Resource,
  ResourceDetail,
  Summary,
} from './types';
import { ARTIFACTS, CLUSTERS, historyFor, inventoryFor, RESOURCES, rawFor } from './mock.fixtures';

const delay = (ms = 180) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/**
 * In-memory client serving the bundled fixtures. It applies the same filters
 * the HTTP endpoints do, so flipping VITE_USE_MOCK to a live backend is a real
 * swap and not a mock that quietly behaves differently.
 */
export class MockFleetApi implements FleetApi {
  async getClusters(): Promise<Cluster[]> {
    await delay();
    return structuredClone(CLUSTERS);
  }

  async getSummary(cluster?: string): Promise<Summary> {
    await delay();
    const scoped = cluster ? RESOURCES.filter((r) => r.cluster === cluster) : RESOURCES;
    const byKind = new Map<string, KindSummary>();
    for (const r of scoped) {
      const k = byKind.get(r.kind) ?? {
        kind: r.kind,
        total: 0,
        ready: 0,
        notReady: 0,
        unknown: 0,
        suspended: 0,
      };
      k.total += 1;
      if (r.ready === 'True') k.ready += 1;
      else if (r.ready === 'False') k.notReady += 1;
      else k.unknown += 1;
      if (r.suspended) k.suspended += 1;
      byKind.set(r.kind, k);
    }
    const kinds = [...byKind.values()].sort((a, b) => a.kind.localeCompare(b.kind));
    return { cluster, total: scoped.length, kinds };
  }

  async getResources(filters: ResourceFilters = {}): Promise<Resource[]> {
    await delay();
    return RESOURCES.filter(
      (r) =>
        (!filters.cluster || r.cluster === filters.cluster) &&
        (!filters.kind || r.kind.toLowerCase() === filters.kind.toLowerCase()) &&
        (!filters.namespace || r.namespace === filters.namespace) &&
        (!filters.ready || r.ready.toLowerCase() === filters.ready.toLowerCase()),
    ).map((r) => ({ ...r }));
  }

  async getResource(
    cluster: string,
    kind: string,
    namespace: string,
    name: string,
  ): Promise<ResourceDetail> {
    await delay(120);
    const found = RESOURCES.find(
      (r) =>
        r.cluster === cluster &&
        r.kind.toLowerCase() === kind.toLowerCase() &&
        r.namespace === namespace &&
        r.name === name,
    );
    if (!found) throw new Error('resource not found');
    return { ...found, raw: rawFor(found), history: historyFor(found) };
  }

  async getArtifacts(filters: ArtifactFilters = {}): Promise<Artifact[]> {
    await delay();
    return ARTIFACTS.filter(
      (a) =>
        (!filters.cluster || a.cluster === filters.cluster) &&
        (!filters.class || a.class.toLowerCase() === filters.class.toLowerCase()),
    ).map((a) => structuredClone(a));
  }

  async getInventory(cluster: string, namespace: string, name: string): Promise<InventoryResponse> {
    await delay(120);
    const entries = inventoryFor(cluster, namespace, name);
    return { cluster, namespace, name, count: entries.length, entries };
  }
}
