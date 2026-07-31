import type { ReactNode } from 'react';
import { routeHash, type Route } from '../lib/route';

/** An in-app navigation link: a real anchor to a hash route, so middle-click,
 *  cmd-click and copy-link work and assistive tech announces a link. Plain
 *  clicks navigate via the hashchange the browser fires — no interception. */
export function RouteLink({
  route,
  className = 'tree__name',
  children,
}: {
  route: Route;
  className?: string;
  children: ReactNode;
}) {
  return (
    <a className={className} href={routeHash(route)}>
      {children}
    </a>
  );
}
