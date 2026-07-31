import { Dialog, Heading, Link, Paragraph, Table, Tag } from '@digdir/designsystemet-react';
import type { AppRelease } from '../lib/appReleases';
import { envLabel } from '../lib/flux';
import { imageLabel, type WorkloadRow } from '../lib/workloads';
import { useDsDialog } from '../hooks/useDsDialog';
import { StageChip } from './StageChip';

export interface ReleaseSelection {
  app: { name: string; namespace: string; kind: string };
  /** Preformatted "Kind namespace/name" of whatever deploys the app. */
  applierLabel?: string;
  release: AppRelease;
  commitUrl: string | null;
  /** All stage columns, for the chips row. */
  envs: string[];
  /** Environments currently running this release. */
  runningEnvs: string[];
  /** The app's workloads scoped to runningEnvs (drift computed within them). */
  workloads: WorkloadRow[];
}

function firstSeenLabel(iso?: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? '—'
    : d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

/** Right-side drawer for one release of an app: revision, commit link, its
 *  stage chips, and — for the environments running it now — the workloads'
 *  container images. */
export function ReleaseDialog({
  selected,
  onClose,
}: {
  selected: ReleaseSelection | null;
  onClose: () => void;
}) {
  const ref = useDsDialog(Boolean(selected), onClose);

  return (
    <Dialog ref={ref} placement="right" closedby="any" onClose={onClose} className="detail">
      {selected && (
        <>
          <Dialog.Block>
            <Heading level={2} data-size="xs">
              {selected.app.name} — {selected.release.shortRev}
            </Heading>
            <Paragraph data-size="sm" data-color="neutral">
              release · {selected.app.namespace} · {selected.app.kind}
              {selected.applierLabel ? ` · deployed by ${selected.applierLabel}` : ''}
            </Paragraph>
          </Dialog.Block>

          <Dialog.Block>
            <dl>
              <dt>Revision</dt>
              <dd>
                <code>{selected.release.revision}</code>
              </dd>
              <dt>First seen</dt>
              <dd>{firstSeenLabel(selected.release.firstSeen)}</dd>
              <dt>Commit</dt>
              <dd>
                {selected.commitUrl ? (
                  <Link href={selected.commitUrl} target="_blank" rel="noreferrer">
                    {selected.commitUrl.replace(/^https?:\/\//, '')} ↗
                  </Link>
                ) : (
                  '—'
                )}
              </dd>
            </dl>
            <div className="release-dialog__chips">
              {selected.envs.map((e) => (
                <StageChip
                  key={e}
                  state={selected.release.chips[e] ?? 'absent'}
                  label={envLabel(e)}
                />
              ))}
            </div>
          </Dialog.Block>

          <Dialog.Block>
            <Heading level={3} data-size="2xs">
              Workloads
            </Heading>
            {selected.runningEnvs.length === 0 ? (
              <Paragraph data-size="sm" data-color="neutral">
                This release is not running in any environment right now — container images are
                only known for running releases.
              </Paragraph>
            ) : selected.workloads.length === 0 ? (
              <Paragraph data-size="sm" data-color="neutral">
                No workloads mirrored for this app — it may apply none, or its workloads carry
                no deployer labels the fleet API sweeps.
              </Paragraph>
            ) : (
              <>
                {/* Which environments run the release is the chips row's job —
                    one Image column suffices. Environments at the same release
                    normally agree; a split (per-env substitution) lists each
                    image with its environments. */}
                <div className="matrix__scroll">
                  <Table data-size="sm">
                    <caption className="sr-only">
                      Container images of {selected.app.name} at release {selected.release.shortRev}
                    </caption>
                    <Table.Head>
                      <Table.Row>
                        <Table.HeaderCell scope="col">Workload</Table.HeaderCell>
                        <Table.HeaderCell scope="col">Container</Table.HeaderCell>
                        <Table.HeaderCell scope="col">Image</Table.HeaderCell>
                      </Table.Row>
                    </Table.Head>
                    <Table.Body>
                      {selected.workloads.flatMap((w) =>
                        w.containers.map((c, i) => {
                          const byImage = new Map<string, string[]>();
                          for (const e of selected.runningEnvs) {
                            const img = w.cells[e]?.images[c];
                            if (img) byImage.set(img, [...(byImage.get(img) ?? []), e]);
                          }
                          const variants = [...byImage.entries()];
                          return (
                            <Table.Row key={`${w.key}|${c}`}>
                              {i === 0 && (
                                <Table.HeaderCell scope="row" rowSpan={w.containers.length}>
                                  <span className="matrix__app">
                                    <strong>{w.name}</strong>
                                    <span className="matrix__app-meta">
                                      {w.kind} · {w.namespace}
                                    </span>
                                  </span>
                                </Table.HeaderCell>
                              )}
                              <Table.Cell>{c}</Table.Cell>
                              <Table.Cell>
                                {variants.length === 0 ? (
                                  <span className="tree__muted">—</span>
                                ) : (
                                  <span className="release-dialog__images">
                                    {variants.map(([image, imgEnvs]) => (
                                      <span key={image} className="release-dialog__image">
                                        <Tag
                                          data-color={variants.length > 1 ? 'warning' : 'neutral'}
                                          data-size="sm"
                                          variant={variants.length > 1 ? 'default' : 'outline'}
                                          className="stage-chip"
                                          title={image}
                                        >
                                          {imageLabel(image)}
                                        </Tag>
                                        {variants.length > 1 && (
                                          <span className="matrix__app-meta">
                                            {imgEnvs.map(envLabel).join(', ')}
                                          </span>
                                        )}
                                      </span>
                                    ))}
                                  </span>
                                )}
                              </Table.Cell>
                            </Table.Row>
                          );
                        }),
                      )}
                    </Table.Body>
                  </Table>
                </div>
                <Paragraph data-size="xs" data-color="neutral">
                  As declared in {selected.runningEnvs.map(envLabel).join(', ')} — the
                  environment{selected.runningEnvs.length === 1 ? '' : 's'} running this release.
                </Paragraph>
              </>
            )}
          </Dialog.Block>
        </>
      )}
    </Dialog>
  );
}
