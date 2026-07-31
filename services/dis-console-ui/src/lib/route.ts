// Page-level navigation is hash-routed so the browser's back/forward buttons
// (and deep links) work; transient UI (drawers, filters, sorting) stays in
// component state on purpose.

export const SECTION_VIEWS = [
  'home',
  'deployments',
  'syncroots',
  'databases',
  'identities',
  'apim',
  'vaults',
] as const;
export type SectionView = (typeof SECTION_VIEWS)[number];

export type Route =
  | { view: SectionView }
  | { view: 'syncroot'; key: string; env?: string; tab?: string }
  | { view: 'kustomization'; cluster: string; namespace: string; name: string };

export function parseRoute(hash: string): Route {
  const parts = hash
    .replace(/^#\/?/, '')
    .split('/')
    .filter(Boolean)
    .map(decodeURIComponent);
  if (parts.length === 0) return { view: 'home' };
  const [head, ...rest] = parts;
  if (head === 'syncroots' && rest.length >= 1) {
    return { view: 'syncroot', key: rest[0], env: rest[1], tab: rest[2] };
  }
  if (head === 'kustomization' && rest.length === 3) {
    return { view: 'kustomization', cluster: rest[0], namespace: rest[1], name: rest[2] };
  }
  if ((SECTION_VIEWS as readonly string[]).includes(head)) {
    return { view: head as SectionView };
  }
  return { view: 'home' };
}

export function routeHash(route: Route): string {
  // Segments are positional, so a tab can only be serialized under an env.
  const segs =
    route.view === 'syncroot'
      ? [
          'syncroots',
          route.key,
          ...(route.env ? [route.env, ...(route.tab ? [route.tab] : [])] : []),
        ]
      : route.view === 'kustomization'
        ? ['kustomization', route.cluster, route.namespace, route.name]
        : [route.view];
  return `#/${segs.map(encodeURIComponent).join('/')}`;
}
