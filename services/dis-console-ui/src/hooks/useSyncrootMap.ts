import { useEffect, useState } from 'react';
import { api } from '../api';
import type { Artifact, Resource } from '../api/types';
import { artifactStatus } from '../lib/artifacts';
import { statusOf, type DeployStatus } from '../lib/flux';
import type { MapEdge } from '../lib/mapLayout';

export interface SyncrootMapNode {
  id: string;
  /** 'syncroot' for the artifact root, otherwise the object kind. */
  kind: string;
  namespace?: string;
  name: string;
  status?: DeployStatus;
  swept: boolean;
}

interface MapData {
  nodes: SyncrootMapNode[];
  edges: MapEdge[];
  loading: boolean;
  error: string | null;
}

const MAX_DEPTH = 5;

/**
 * Assembles the full deployment graph of one syncroot by walking every
 * Kustomization's inventory (recursively for nested Kustomizations) and
 * attaching DIS children via the `parent` relation. Only declared objects
 * appear — the inventory has no runtime children (ReplicaSets, Pods), which
 * keeps the map at the level people reason about.
 */
export function useSyncrootMap(artifact: Artifact | undefined, resources: Resource[]): MapData {
  const [state, setState] = useState<MapData>({ nodes: [], edges: [], loading: false, error: null });

  useEffect(() => {
    if (!artifact) {
      setState({ nodes: [], edges: [], loading: false, error: null });
      return;
    }
    let cancelled = false;
    setState((s) => ({ ...s, loading: true, error: null }));

    const cluster = artifact.cluster;
    const nodes = new Map<string, SyncrootMapNode>();
    const edges: MapEdge[] = [];
    const idOf = (kind: string, ns: string | undefined, name: string) => `${kind}|${ns ?? ''}|${name}`;
    const findRes = (kind: string, ns: string, name: string) =>
      resources.find(
        (r) => r.cluster === cluster && r.kind === kind && r.namespace === ns && r.name === name,
      );

    const rootId = `syncroot|${artifact.namespace}|${artifact.name}`;
    nodes.set(rootId, {
      id: rootId,
      kind: 'syncroot',
      namespace: artifact.namespace,
      name: artifact.name,
      status: artifactStatus(artifact),
      swept: true,
    });

    const visited = new Set<string>();

    const walkKust = async (ns: string, name: string, fromId: string, depth: number): Promise<void> => {
      const id = idOf('Kustomization', ns, name);
      if (!nodes.has(id)) {
        const res = findRes('Kustomization', ns, name);
        nodes.set(id, {
          id,
          kind: 'Kustomization',
          namespace: ns,
          name,
          status: res ? statusOf(res) : undefined,
          swept: Boolean(res),
        });
      }
      edges.push({ from: fromId, to: id });
      if (visited.has(id) || depth >= MAX_DEPTH) return;
      visited.add(id);

      let entries;
      try {
        entries = (await api.getInventory(cluster, ns, name)).entries;
      } catch {
        return; // a missing inventory just ends this branch
      }

      const nested: Promise<void>[] = [];
      for (const e of entries) {
        if (e.kind === 'Kustomization') {
          nested.push(walkKust(e.namespace ?? ns, e.name, id, depth + 1));
          continue;
        }
        const eid = idOf(e.kind, e.namespace, e.name);
        if (!nodes.has(eid)) {
          nodes.set(eid, {
            id: eid,
            kind: e.kind,
            namespace: e.namespace,
            name: e.name,
            status: e.resource ? statusOf(e.resource) : undefined,
            swept: Boolean(e.resource),
          });
        }
        edges.push({ from: id, to: eid });

        // DIS children (e.g. Databases under a DatabaseServer) are not
        // inventory entries — they hang off the parent relation.
        const parentNs = e.resource?.namespace ?? e.namespace;
        for (const child of resources.filter(
          (r) =>
            r.cluster === cluster &&
            r.namespace === parentNs &&
            r.parent?.kind === e.kind &&
            r.parent?.name === e.name,
        )) {
          const cid = idOf(child.kind, child.namespace, child.name);
          if (!nodes.has(cid)) {
            nodes.set(cid, {
              id: cid,
              kind: child.kind,
              namespace: child.namespace,
              name: child.name,
              status: statusOf(child),
              swept: true,
            });
          }
          edges.push({ from: eid, to: cid });
        }
      }
      await Promise.all(nested);
    };

    Promise.all(artifact.kustomizations.map((k) => walkKust(k.namespace, k.name, rootId, 1)))
      .then(() => {
        if (cancelled) return;
        // Keep it a tree: the first edge into a node wins.
        const seenTo = new Set<string>();
        const treeEdges = edges.filter((e) => {
          if (seenTo.has(e.to)) return false;
          seenTo.add(e.to);
          return true;
        });
        setState({ nodes: [...nodes.values()], edges: treeEdges, loading: false, error: null });
      })
      .catch((e: unknown) => {
        if (!cancelled)
          setState({
            nodes: [],
            edges: [],
            loading: false,
            error: e instanceof Error ? e.message : String(e),
          });
      });

    return () => {
      cancelled = true;
    };
  }, [artifact, resources]);

  return state;
}
