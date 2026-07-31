import { useMemo } from 'react';
import { Dialog, Heading, Link, Paragraph, Spinner } from '@digdir/designsystemet-react';
import type { Resource } from '../api/types';
import { envLabel, envOf, statusOf } from '../lib/flux';
import { useDsDialog } from '../hooks/useDsDialog';
import type { ResourceRef } from '../hooks/useResourceDetail';
import { useResourceDetail } from '../hooks/useResourceDetail';
import { useSourceLink } from '../hooks/useSourceLink';
import { appliedByFromRaw, applierKindFromRaw, managedByFromRaw, ownerFromRaw } from '../lib/sourceLink';
import { StatusTag } from './StatusTag';

interface Props {
  selected: ResourceRef | null;
  /** The full fleet list — used to show what a Kustomization applies. */
  resources: Resource[];
  /** Swap the drawer to another resource (parent / applier / child). */
  onSelect: (ref: ResourceRef) => void;
  onClose: () => void;
}

/** A single, controlled right-side drawer that lazily loads the selected
 *  resource's detail (including its raw object). */
export function DetailDialog({ selected, resources, onSelect, onClose }: Props) {
  const ref = useDsDialog(Boolean(selected), onClose);
  const { resource, loading, error } = useResourceDetail(selected);
  const { link: source } = useSourceLink(resource);
  const appliedBy = resource?.appliedBy ?? (resource ? appliedByFromRaw(resource.raw) : null);
  // Roots have no kustomize owner; azapi-provisioned ones carry Azure's
  // clusterconfig labels instead.
  const managedBy = !appliedBy && resource ? managedByFromRaw(resource.raw) : null;

  // The direct creator, when a controller made this object (an otel-operator
  // collector Deployment, an azapi root owned by its FluxConfig).
  const owner = resource ? ownerFromRaw(resource.raw) : null;
  const ownerSwept =
    owner && selected
      ? resources.find(
          (r) =>
            r.cluster === selected.cluster &&
            r.kind === owner.kind &&
            r.namespace === selected.namespace &&
            r.name === owner.name,
        )
      : undefined;

  // The applier as an openable reference: appliedBy carries only name +
  // namespace, so the kind comes from the raw object's deployer markers,
  // falling back to whichever app exists in the sweep under that identity.
  const applier = useMemo(() => {
    if (!appliedBy || !selected) return null;
    const ns = appliedBy.namespace ?? selected.namespace;
    const inSweep = (kind: string) =>
      resources.find(
        (r) =>
          r.cluster === selected.cluster &&
          r.kind === kind &&
          r.namespace === ns &&
          r.name === appliedBy.name,
      );
    const kind =
      (resource ? applierKindFromRaw(resource.raw) : null) ??
      (inSweep('Kustomization') ? 'Kustomization' : inSweep('HelmRelease') ? 'HelmRelease' : null);
    return {
      kind,
      namespace: ns,
      name: appliedBy.name,
      swept: kind ? Boolean(inSweep(kind)) : false,
    };
  }, [appliedBy, selected, resource, resources]);

  // What this Kustomization applies, from the normalized list (same cluster).
  const applies = useMemo(() => {
    if (!selected || selected.kind !== 'Kustomization') return [];
    return resources.filter(
      (r) =>
        r.cluster === selected.cluster &&
        r.appliedBy?.name === selected.name &&
        r.appliedBy?.namespace === selected.namespace,
    );
  }, [resources, selected]);

  return (
    <Dialog ref={ref} placement="right" closedby="any" onClose={onClose} className="detail">
      {selected && (
        <>
          <Dialog.Block>
            <Heading level={2} data-size="xs">
              {selected.name}
            </Heading>
            <Paragraph data-size="sm" data-color="neutral">
              {selected.kind} · {selected.namespace} · {envLabel(envOf(selected.cluster))}
            </Paragraph>
          </Dialog.Block>

          <Dialog.Block>
            {loading && <Spinner aria-label="Loading resource details" data-size="sm" />}
            {error && <Paragraph data-color="danger">{error}</Paragraph>}
            {resource && (
              <dl>
                <dt>Status</dt>
                <dd>
                  <StatusTag status={statusOf(resource)} />
                </dd>
                <dt>Cluster</dt>
                <dd>{resource.cluster}</dd>
                {applier && (
                  <>
                    <dt>Deployed by</dt>
                    <dd>
                      {applier.kind && applier.swept ? (
                        <button
                          type="button"
                          className="tree__name"
                          onClick={() =>
                            onSelect({
                              cluster: selected.cluster,
                              kind: applier.kind!,
                              namespace: applier.namespace,
                              name: applier.name,
                            })
                          }
                        >
                          {applier.kind} {applier.namespace}/{applier.name}
                        </button>
                      ) : (
                        `${applier.kind ? `${applier.kind} ` : ''}${applier.namespace}/${applier.name}`
                      )}
                    </dd>
                  </>
                )}
                {owner && (
                  <>
                    <dt>Owner</dt>
                    <dd>
                      {ownerSwept ? (
                        <button
                          type="button"
                          className="tree__name"
                          onClick={() =>
                            onSelect({
                              cluster: ownerSwept.cluster,
                              kind: ownerSwept.kind,
                              namespace: ownerSwept.namespace,
                              name: ownerSwept.name,
                            })
                          }
                        >
                          {owner.kind} {owner.name}
                        </button>
                      ) : (
                        `${owner.kind} ${owner.name}`
                      )}
                    </dd>
                  </>
                )}
                {resource.parent && (
                  <>
                    <dt>Parent</dt>
                    <dd>
                      <button
                        type="button"
                        className="tree__name"
                        onClick={() =>
                          onSelect({
                            cluster: selected.cluster,
                            kind: resource.parent!.kind,
                            namespace: selected.namespace,
                            name: resource.parent!.name,
                          })
                        }
                      >
                        {resource.parent.kind} {resource.parent.name}
                      </button>
                    </dd>
                  </>
                )}
                {managedBy && (
                  <>
                    <dt>Managed by</dt>
                    <dd>Azure fluxConfiguration {managedBy.name}</dd>
                  </>
                )}
                <dt>Revision</dt>
                <dd>{resource.revision ?? '—'}</dd>
                {source && (
                  <>
                    <dt>Source</dt>
                    <dd>
                      <Link href={source.commitUrl ?? source.repoUrl} target="_blank" rel="noreferrer">
                        {source.label}
                        {source.shortSha ? ` @ ${source.shortSha}` : ''}
                      </Link>
                    </dd>
                  </>
                )}
                <dt>Reason</dt>
                <dd>{resource.reason ?? '—'}</dd>
                <dt>Message</dt>
                <dd>{resource.message ?? '—'}</dd>
                <dt>Last transition</dt>
                <dd>{resource.lastTransition ? new Date(resource.lastTransition).toLocaleString() : '—'}</dd>
                <dt>Suspended</dt>
                <dd>{resource.suspended ? 'Yes' : 'No'}</dd>
              </dl>
            )}
          </Dialog.Block>

          {applies.length > 0 && (
            <Dialog.Block>
              <Heading level={3} data-size="2xs">
                Applies
              </Heading>
              <ul className="detail__applies">
                {applies.map((r) => (
                  <li key={`${r.kind}/${r.namespace}/${r.name}`} className="detail__applies-row">
                    <span className="detail__applies-kind">{r.kind}</span>
                    <button
                      type="button"
                      className="tree__name"
                      onClick={() =>
                        onSelect({
                          cluster: r.cluster,
                          kind: r.kind,
                          namespace: r.namespace,
                          name: r.name,
                        })
                      }
                    >
                      <strong>{r.name}</strong>
                    </button>
                    <StatusTag status={statusOf(r)} />
                  </li>
                ))}
              </ul>
            </Dialog.Block>
          )}

          {resource?.raw != null && (
            <Dialog.Block>
              <Heading level={3} data-size="2xs">
                Raw object
              </Heading>
              <pre>{JSON.stringify(resource.raw, null, 2)}</pre>
            </Dialog.Block>
          )}
        </>
      )}
    </Dialog>
  );
}
