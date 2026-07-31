import { useEffect, useState } from 'react';
import { Button, Dialog, Heading, Link, Paragraph, Spinner } from '@digdir/designsystemet-react';
import type { Artifact } from '../api/types';
import { envLabel, envOf, statusOf } from '../lib/flux';
import { artifactStatus, artifactTag, classLabel, shortDigest } from '../lib/artifacts';
import { linkFromOrigin } from '../lib/sourceLink';
import { useDsDialog } from '../hooks/useDsDialog';
import { useInventory, type KustomizationRef } from '../hooks/useInventory';
import { StatusTag } from './StatusTag';

interface Props {
  selected: Artifact | null;
  onClose: () => void;
}

/** Right-side drawer for one base-layer artifact: identity, origin commit
 *  link, the Kustomizations building from it, and their inventories. */
export function ArtifactDialog({ selected, onClose }: Props) {
  const ref = useDsDialog(Boolean(selected), onClose);
  const [inventoryRef, setInventoryRef] = useState<KustomizationRef | null>(null);
  const origin = selected ? linkFromOrigin(selected.originSource, selected.originRevision) : null;

  useEffect(() => {
    if (!selected) setInventoryRef(null);
  }, [selected]);

  return (
    <Dialog ref={ref} placement="right" closedby="any" onClose={onClose} className="detail">
      {selected && (
        <>
          <Dialog.Block>
            <Heading level={2} data-size="xs">
              {selected.name}
            </Heading>
            <Paragraph data-size="sm" data-color="neutral">
              {classLabel(selected.class)}
              {selected.owner ? ` · ${selected.owner}` : ''} · {envLabel(envOf(selected.cluster))}
            </Paragraph>
          </Dialog.Block>

          <Dialog.Block>
            <dl>
              <dt>Status</dt>
              <dd>
                <StatusTag status={artifactStatus(selected)} />
              </dd>
              <dt>Cluster</dt>
              <dd>{selected.cluster}</dd>
              <dt>URL</dt>
              <dd>{selected.url}</dd>
              <dt>Fetched</dt>
              <dd>
                {shortDigest(selected.revision) || '—'}
                {artifactTag(selected.revision) ? ` (tag ${artifactTag(selected.revision)})` : ''}
              </dd>
              {origin && (
                <>
                  <dt>Origin</dt>
                  <dd>
                    <Link href={origin.commitUrl ?? origin.repoUrl} target="_blank" rel="noreferrer">
                      {origin.label}
                      {origin.shortSha ? ` @ ${origin.shortSha}` : ''}
                    </Link>
                  </dd>
                </>
              )}
              <dt>Reason</dt>
              <dd>{selected.reason ?? '—'}</dd>
            </dl>
          </Dialog.Block>

          <Dialog.Block>
            <Heading level={3} data-size="2xs">
              Deployed by
            </Heading>
            <ul className="detail__applies">
              {selected.kustomizations.length === 0 && (
                <li className="detail__applies-row">No Kustomization builds from this artifact.</li>
              )}
              {selected.kustomizations.map((k) => {
                const applied = shortDigest(k.revision);
                const fetched = shortDigest(selected.revision);
                const lagging = Boolean(applied && fetched && applied !== fetched);
                const kRef: KustomizationRef = {
                  cluster: selected.cluster,
                  namespace: k.namespace,
                  name: k.name,
                };
                const isOpen =
                  inventoryRef?.cluster === kRef.cluster &&
                  inventoryRef?.namespace === kRef.namespace &&
                  inventoryRef?.name === kRef.name;
                return (
                  <li key={`${k.namespace}/${k.name}`}>
                    <div className="detail__applies-row">
                      <span className="detail__applies-kind">Kustomization</span>
                      <strong>{k.name}</strong>
                      <StatusTag status={artifactStatus(k)} />
                      <span className="dis__rg">
                        applied {applied || '—'}
                        {lagging ? ' (behind)' : ''}
                      </span>
                      <Button
                        variant="tertiary"
                        data-size="sm"
                        onClick={() => setInventoryRef(isOpen ? null : kRef)}
                      >
                        {isOpen ? 'Hide inventory' : 'Inventory'}
                      </Button>
                    </div>
                    {isOpen && <InventoryList kRef={kRef} />}
                  </li>
                );
              })}
            </ul>
          </Dialog.Block>
        </>
      )}
    </Dialog>
  );
}

function InventoryList({ kRef }: { kRef: KustomizationRef }) {
  const { inventory, loading, error } = useInventory(kRef);
  if (loading) return <Spinner aria-label="Loading inventory" data-size="sm" />;
  if (error) return <Paragraph data-color="danger">{error}</Paragraph>;
  if (!inventory || inventory.entries.length === 0) {
    return <Paragraph data-size="sm">No inventory recorded.</Paragraph>;
  }
  return (
    <ul className="detail__inventory">
      {inventory.entries.map((e) => (
        <li key={`${e.kind}/${e.namespace ?? ''}/${e.name}`} className="detail__applies-row">
          <span className="detail__applies-kind">{e.kind}</span>
          <span>
            {e.namespace ? `${e.namespace}/` : ''}
            {e.name}
          </span>
          {e.resource ? <StatusTag status={statusOf(e.resource)} /> : null}
        </li>
      ))}
    </ul>
  );
}
