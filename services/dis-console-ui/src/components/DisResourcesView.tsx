import { useMemo } from 'react';
import { Heading, Link, Tag } from '@digdir/designsystemet-react';
import type { Resource } from '../api/types';
import { statusOf } from '../lib/flux';
import { statusStyle } from '../lib/statusColor';
import { buildDisResources } from '../lib/disResources';
import { portalUrl, resourceGroupOf } from '../lib/azure';

const PORTAL_TENANT = import.meta.env.VITE_AZURE_PORTAL_TENANT || undefined;

interface Props {
  resources: Resource[];
  cluster: string;
  kinds: readonly string[];
  title: string;
}

/** A per-product DIS resource view: the resources of one product family
 *  (e.g. Databases) for the selected cluster, grouped by namespace, each with
 *  its Azure Portal deep-link. */
export function DisResourcesView({ resources, cluster, kinds, title }: Props) {
  const groups = useMemo(
    () => (cluster ? buildDisResources(resources, { cluster, kinds }) : []),
    [resources, cluster, kinds],
  );
  const count = useMemo(
    () =>
      groups.reduce((n, g) => n + g.nodes.reduce((m, node) => m + 1 + node.children.length, 0), 0),
    [groups],
  );

  return (
    <div className="dis">
      <div className="dis__head">
        <Heading level={2} data-size="sm">
          {title}
        </Heading>
        <span className="matrix__count">
          {count} in {cluster || 'this selection'}
        </span>
      </div>

      {groups.length === 0 ? (
        <p className="dis__empty">
          No {title.toLowerCase()} for {cluster || 'this selection'}.
        </p>
      ) : (
        groups.map((group) => (
          <section key={group.namespace} className="dis__ns">
            <h3 className="dis__ns-title">{group.namespace}</h3>
            <ul className="dis__list">
              {group.nodes.map((node) => (
                <li key={`${node.resource.kind}/${node.resource.name}`}>
                  <DisRow resource={node.resource} />
                  {node.children.length > 0 && (
                    <ul className="dis__children">
                      {node.children.map((c) => (
                        <li key={`${c.kind}/${c.name}`}>
                          <DisRow resource={c} child />
                        </li>
                      ))}
                    </ul>
                  )}
                </li>
              ))}
            </ul>
          </section>
        ))
      )}
    </div>
  );
}

function DisRow({ resource, child }: { resource: Resource; child?: boolean }) {
  const style = statusStyle(statusOf(resource));
  const rg = resource.azureResourceId ? resourceGroupOf(resource.azureResourceId) : '';
  return (
    <div className={child ? 'dis__row dis__row--child' : 'dis__row'}>
      <span className="dis__kind">{resource.kind}</span>
      <strong className="dis__name">{resource.name}</strong>
      <Tag data-color={style.color} data-size="sm" variant={style.variant}>
        {style.label}
      </Tag>
      {rg ? <span className="dis__rg">{rg}</span> : null}
      {resource.azureResourceId ? (
        <Link
          href={portalUrl(resource.azureResourceId, PORTAL_TENANT)}
          target="_blank"
          rel="noreferrer"
          className="dis__portal"
        >
          Azure Portal ↗
        </Link>
      ) : (
        <span className="dis__norsrc">no direct Azure resource</span>
      )}
    </div>
  );
}
