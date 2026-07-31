import { Table, Tag } from '@digdir/designsystemet-react';
import { envLabel } from '../lib/flux';
import { imageLabel, type AppWorkloadRow } from '../lib/workloads';

/** Workloads × environments of declared container images: one row per
 *  workload container, a tag per environment — yellow when the container's
 *  image drifts across the shown environments, red/blue by workload status. */
export function WorkloadsTable({
  workloads,
  envs,
  caption,
}: {
  workloads: AppWorkloadRow[];
  envs: string[];
  caption: string;
}) {
  return (
    <div className="matrix__scroll">
      <Table hover data-size="sm">
        <caption className="sr-only">{caption}</caption>
        <Table.Head>
          <Table.Row>
            <Table.HeaderCell scope="col">Workload</Table.HeaderCell>
            <Table.HeaderCell scope="col">Container</Table.HeaderCell>
            {envs.map((e) => (
              <Table.HeaderCell key={e} scope="col">
                {envLabel(e)}
              </Table.HeaderCell>
            ))}
          </Table.Row>
        </Table.Head>
        <Table.Body>
          {workloads.length === 0 ? (
            <Table.Row>
              <Table.Cell colSpan={envs.length + 2}>No workloads mirrored.</Table.Cell>
            </Table.Row>
          ) : (
            workloads.flatMap((w) =>
              w.containers.map((c, i) => (
                <Table.Row key={`${w.app ?? ''}|${w.key}|${c}`}>
                  {i === 0 && (
                    <Table.HeaderCell scope="row" rowSpan={w.containers.length}>
                      <span className="matrix__app">
                        <strong>{w.name}</strong>
                        <span className="matrix__app-meta">
                          {w.kind} · {w.namespace}
                          {w.app && w.app !== w.name ? ` · app ${w.app}` : ''}
                        </span>
                      </span>
                    </Table.HeaderCell>
                  )}
                  <Table.Cell>{c}</Table.Cell>
                  {envs.map((e) => {
                    const cell = w.cells[e];
                    const image = cell?.images[c];
                    if (!cell || !image) {
                      return (
                        <Table.Cell key={e}>
                          <span className="tree__muted">—</span>
                        </Table.Cell>
                      );
                    }
                    const color =
                      cell.status === 'failed'
                        ? 'danger'
                        : w.driftContainers.has(c)
                          ? 'warning'
                          : cell.status === 'reconciling'
                            ? 'info'
                            : 'neutral';
                    return (
                      <Table.Cell key={e}>
                        <Tag
                          data-color={color}
                          data-size="sm"
                          variant={color === 'neutral' ? 'outline' : 'default'}
                          className="stage-chip"
                          title={image}
                        >
                          {imageLabel(image)}
                        </Tag>
                      </Table.Cell>
                    );
                  })}
                </Table.Row>
              )),
            )
          )}
        </Table.Body>
      </Table>
    </div>
  );
}
