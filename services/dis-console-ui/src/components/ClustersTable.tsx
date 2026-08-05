import { Table, Tag } from '@digdir/designsystemet-react';
import type { Cluster } from '../api/types';
import { envLabel } from '../lib/flux';

function relative(iso: string): string {
  if (!iso) return '—';
  const ms = new Date(iso).getTime();
  if (Number.isNaN(ms)) return '—';
  const secs = Math.max(0, Math.round((Date.now() - ms) / 1000));
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.round(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.round(mins / 60);
  if (hrs < 48) return `${hrs}h ago`;
  return `${Math.round(hrs / 24)}d ago`;
}

/** The fleet's clusters and their sync freshness (backed by /api/clusters). */
export function ClustersTable({ clusters }: { clusters: Cluster[] }) {
  const sorted = [...clusters].sort((a, b) => a.cluster.localeCompare(b.cluster));
  return (
    <div className="matrix__scroll">
      <Table hover data-size="sm">
        <caption className="sr-only">Synced clusters and their freshness</caption>
        <Table.Head>
          <Table.Row>
            <Table.HeaderCell scope="col">Cluster</Table.HeaderCell>
            <Table.HeaderCell scope="col">Environment</Table.HeaderCell>
            <Table.HeaderCell scope="col">Resources</Table.HeaderCell>
            <Table.HeaderCell scope="col">Last sweep</Table.HeaderCell>
            <Table.HeaderCell scope="col">Last sync</Table.HeaderCell>
            <Table.HeaderCell scope="col">State</Table.HeaderCell>
          </Table.Row>
        </Table.Head>
        <Table.Body>
          {sorted.map((c) => (
            <Table.Row key={c.cluster}>
              <Table.HeaderCell scope="row">{c.cluster}</Table.HeaderCell>
              <Table.Cell>{c.environment ? envLabel(c.environment) : '—'}</Table.Cell>
              <Table.Cell>{c.resourceCount}</Table.Cell>
              <Table.Cell>{relative(c.lastSweepAt)}</Table.Cell>
              <Table.Cell>{relative(c.lastSyncedAt)}</Table.Cell>
              <Table.Cell>
                <Tag
                  data-color={c.stale ? 'danger' : 'success'}
                  data-size="sm"
                  variant={c.stale ? 'default' : 'outline'}
                >
                  {c.stale ? 'Stale' : 'OK'}
                </Tag>
              </Table.Cell>
            </Table.Row>
          ))}
        </Table.Body>
      </Table>
    </div>
  );
}
