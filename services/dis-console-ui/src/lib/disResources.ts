import type { Resource } from '../api/types';
import { ENV_ORDER, envOf, isDisKind, tenantOf, type Environment } from './flux';

export interface DisNode {
  resource: Resource;
  /** Nested resources (e.g. a DatabaseServer's Databases, an Api's ApiVersions). */
  children: Resource[];
}

export interface DisNamespaceGroup {
  namespace: string;
  nodes: DisNode[];
}

export interface BuildDisOptions {
  cluster?: string;
  /** Restrict to these kinds (one DIS product family); default = all DIS kinds. */
  kinds?: readonly string[];
}

/** Tenants that own DIS resources, sorted. */
export function disTenantsOf(resources: Resource[]): string[] {
  return [
    ...new Set(resources.filter((r) => isDisKind(r.kind)).map((r) => tenantOf(r.cluster))),
  ].sort();
}

/** Environments with DIS resources for a tenant, in promotion order. */
export function disEnvsOf(resources: Resource[], tenant?: string): string[] {
  const present = new Set(
    resources
      .filter((r) => isDisKind(r.kind) && (!tenant || tenantOf(r.cluster) === tenant))
      .map((r) => envOf(r.cluster)),
  );
  const known = ENV_ORDER.filter((e) => present.has(e));
  const extra = [...present].filter((e) => e && !ENV_ORDER.includes(e as Environment)).sort();
  return [...known, ...extra];
}

/**
 * Shape the DIS resources of one cluster into namespace groups, each a small
 * tree: a parent resource (DatabaseServer, Api, …) with the resources that
 * point at it via `parent` nested underneath. Children whose parent isn't in
 * the data are surfaced at the top level so nothing is hidden. Pass `kinds` to
 * restrict to one product family (e.g. Databases = DatabaseServer + Database).
 */
export function buildDisResources(
  resources: Resource[],
  opts: BuildDisOptions = {},
): DisNamespaceGroup[] {
  const dis = resources.filter(
    (r) =>
      isDisKind(r.kind) &&
      (!opts.kinds || opts.kinds.includes(r.kind)) &&
      (!opts.cluster || r.cluster === opts.cluster),
  );

  const byNs = new Map<string, Resource[]>();
  for (const r of dis) {
    const list = byNs.get(r.namespace) ?? [];
    list.push(r);
    byNs.set(r.namespace, list);
  }

  const groups: DisNamespaceGroup[] = [];
  for (const namespace of [...byNs.keys()].sort()) {
    const rs = byNs.get(namespace) ?? [];
    const parents = rs.filter((r) => !r.parent);
    const children = rs.filter((r) => r.parent);

    const nodes: DisNode[] = parents
      .map((p) => ({
        resource: p,
        children: children
          .filter((c) => c.parent?.kind === p.kind && c.parent?.name === p.name)
          .sort((a, b) => a.name.localeCompare(b.name)),
      }))
      .sort(
        (a, b) =>
          a.resource.kind.localeCompare(b.resource.kind) ||
          a.resource.name.localeCompare(b.resource.name),
      );

    const claimed = new Set(nodes.flatMap((n) => n.children.map((c) => `${c.kind}/${c.name}`)));
    for (const c of children) {
      if (!claimed.has(`${c.kind}/${c.name}`)) nodes.push({ resource: c, children: [] });
    }

    groups.push({ namespace, nodes });
  }
  return groups;
}
