import { useCallback, useMemo, useSyncExternalStore } from 'react';
import { parseRoute, routeHash, type Route } from '../lib/route';

function subscribe(onChange: () => void) {
  window.addEventListener('hashchange', onChange);
  return () => window.removeEventListener('hashchange', onChange);
}

/** The current hash route + a navigate function. Plain navigation pushes a
 *  history entry (browser back works); `replace` swaps the current one (used
 *  for in-page tweaks like the environment picker). */
export function useHashRoute(): [Route, (route: Route, opts?: { replace?: boolean }) => void] {
  const hash = useSyncExternalStore(subscribe, () => window.location.hash);
  const route = useMemo(() => parseRoute(hash), [hash]);
  const navigate = useCallback((r: Route, opts?: { replace?: boolean }) => {
    const h = routeHash(r);
    if (opts?.replace) window.location.replace(h);
    else window.location.hash = h;
  }, []);
  return [route, navigate];
}
