import { useMemo, useState } from 'react';
import {
  Button,
  Field,
  Heading,
  Label,
  Paragraph,
  Select,
  Spinner,
  Table,
} from '@digdir/designsystemet-react';
import { ArrowLeft } from 'lucide-react';
import type { Resource } from '../api/types';
import type { ResourceRef } from '../hooks/useResourceDetail';
import { useInventory } from '../hooks/useInventory';
import { STATUS_SEVERITY, shortRev, statusOf, type DeployStatus } from '../lib/flux';
import { statusStyle } from '../lib/statusColor';
import { sortRows, toggleSort, type SortCol, type SortState } from '../lib/tableSort';
import { RouteLink } from './RouteLink';
import { SortableTh } from './SortableTh';
import { StatusTag } from './StatusTag';

export interface KustomizationRef {
  cluster: string;
  namespace: string;
  name: string;
}

interface Props {
  kust: KustomizationRef;
  resources: Resource[];
  onBack: () => void;
  onSelectResource: (ref: ResourceRef) => void;
}

interface Row {
  kind: string;
  namespace: string;
  name: string;
  status: DeployStatus;
  swept: boolean;
  revision?: string;
}

/** One Kustomization's page: everything it applied (its status.inventory —
 *  including kinds the agent doesn't sweep), sortable and kind-filterable.
 *  Swept rows open the detail drawer; nested Kustomizations open their page. */
export function KustomizationPage({ kust, resources, onBack, onSelectResource }: Props) {
  const { inventory, loading, error } = useInventory(kust);
  const [kind, setKind] = useState('');
  const [status, setStatus] = useState('');
  const [sort, setSort] = useState<SortState>({ col: 'kind', dir: 'asc' });

  const self = resources.find(
    (r) =>
      r.cluster === kust.cluster &&
      r.kind === 'Kustomization' &&
      r.namespace === kust.namespace &&
      r.name === kust.name,
  );

  const rows: Row[] = useMemo(
    () =>
      (inventory?.entries ?? []).map((e) => ({
        kind: e.kind,
        namespace: e.namespace ?? '',
        name: e.name,
        status: e.resource ? statusOf(e.resource) : 'unknown',
        swept: Boolean(e.resource),
        revision: e.resource?.revision,
      })),
    [inventory],
  );

  const kinds = useMemo(() => [...new Set(rows.map((r) => r.kind))].sort(), [rows]);
  const activeKind = kinds.includes(kind) ? kind : '';
  const statuses = useMemo(
    () =>
      [...new Set(rows.filter((r) => r.swept).map((r) => r.status))].sort(
        (a, b) => STATUS_SEVERITY[b] - STATUS_SEVERITY[a],
      ),
    [rows],
  );
  const activeStatus = statuses.includes(status as DeployStatus) ? status : '';
  const shown = useMemo(
    () =>
      sortRows(
        rows.filter(
          (r) =>
            (!activeKind || r.kind === activeKind) &&
            (!activeStatus || (r.swept && r.status === activeStatus)),
        ),
        sort,
      ),
    [rows, activeKind, activeStatus, sort],
  );
  const onSort = (col: SortCol) => setSort((s) => toggleSort(s, col));

  return (
    <div className="syncroots">
      <div className="syncroot-page__head">
        <Button variant="tertiary" data-size="sm" onClick={onBack}>
          <ArrowLeft size="1em" aria-hidden /> Back
        </Button>
        <Heading level={2} data-size="sm">
          {kust.namespace}/{kust.name}
        </Heading>
        {self && <StatusTag status={statusOf(self)} />}
        {self?.revision && <span className="cell__rev">{shortRev(self.revision)}</span>}
        <Button
          variant="tertiary"
          data-size="sm"
          onClick={() =>
            onSelectResource({
              cluster: kust.cluster,
              kind: 'Kustomization',
              namespace: kust.namespace,
              name: kust.name,
            })
          }
        >
          Details
        </Button>
      </div>

      <div className="matrix__filters">
        <Field>
          <Label data-size="sm">Kind</Label>
          <Select value={activeKind} onChange={(e) => setKind(e.target.value)} data-size="sm" width="auto">
            <Select.Option value="">All kinds</Select.Option>
            {kinds.map((k) => (
              <Select.Option key={k} value={k}>
                {k}
              </Select.Option>
            ))}
          </Select>
        </Field>
        <Field>
          <Label data-size="sm">Status</Label>
          <Select value={activeStatus} onChange={(e) => setStatus(e.target.value)} data-size="sm" width="auto">
            <Select.Option value="">All statuses</Select.Option>
            {statuses.map((s) => (
              <Select.Option key={s} value={s}>
                {statusStyle(s).label}
              </Select.Option>
            ))}
          </Select>
        </Field>
        <span className="matrix__count">
          {activeKind || activeStatus ? `${shown.length} of ${rows.length}` : `${shown.length}`} applied
          object{shown.length === 1 ? '' : 's'}
        </span>
      </div>

      {loading && <Spinner aria-label="Loading inventory" data-size="sm" />}
      {error && <Paragraph data-color="danger">{error}</Paragraph>}
      {!loading && !error && rows.length === 0 && (
        <Paragraph data-color="neutral">
          No inventory recorded for this Kustomization yet.
        </Paragraph>
      )}

      {shown.length > 0 && (
        <div className="matrix__scroll">
          <Table stickyHeader hover data-size="sm">
            <caption className="sr-only">Objects applied by this Kustomization</caption>
            <Table.Head>
              <Table.Row>
                <SortableTh col="kind" label="Kind" sort={sort} onSort={onSort} />
                <SortableTh col="namespace" label="Namespace" sort={sort} onSort={onSort} />
                <SortableTh col="name" label="Name" sort={sort} onSort={onSort} />
                <SortableTh col="status" label="Status" sort={sort} onSort={onSort} />
                <Table.HeaderCell scope="col">Revision</Table.HeaderCell>
              </Table.Row>
            </Table.Head>
            <Table.Body>
              {shown.map((r) => (
                <Table.Row key={`${r.kind}/${r.namespace}/${r.name}`}>
                  <Table.Cell>{r.kind}</Table.Cell>
                  <Table.Cell>{r.namespace || '—'}</Table.Cell>
                  <Table.Cell>
                    {r.kind === 'Kustomization' ? (
                      <RouteLink
                        route={{
                          view: 'kustomization',
                          cluster: kust.cluster,
                          namespace: r.namespace || kust.namespace,
                          name: r.name,
                        }}
                      >
                        {r.name}
                      </RouteLink>
                    ) : r.swept ? (
                      <button
                        type="button"
                        className="tree__name"
                        onClick={() =>
                          onSelectResource({
                            cluster: kust.cluster,
                            kind: r.kind,
                            namespace: r.namespace,
                            name: r.name,
                          })
                        }
                      >
                        {r.name}
                      </button>
                    ) : (
                      r.name
                    )}
                  </Table.Cell>
                  <Table.Cell>
                    {r.swept ? <StatusTag status={r.status} /> : <span className="tree__muted">not swept</span>}
                  </Table.Cell>
                  <Table.Cell>
                    <span className="cell__rev">{shortRev(r.revision)}</span>
                  </Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table>
        </div>
      )}
    </div>
  );
}
