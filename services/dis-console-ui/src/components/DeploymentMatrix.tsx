import { useMemo, useState } from 'react';
import { Field, Label, Select, Table, Tabs, Tag } from '@digdir/designsystemet-react';
import type { Resource } from '../api/types';
import { envLabel } from '../lib/flux';
import { buildMatrix, kindsOf, namespacesOf, tenantsOf, type MatrixCell } from '../lib/matrix';
import type { ResourceRef } from '../hooks/useResourceDetail';
import { StatusCell } from './StatusCell';

interface Props {
  resources: Resource[];
  onSelectCell: (ref: ResourceRef) => void;
}

/** The deployment matrix: apps as rows, environments as stage columns,
 *  scoped to one tenant at a time (tenant derived from the cluster id). */
export function DeploymentMatrix({ resources, onSelectCell }: Props) {
  const [tenant, setTenant] = useState('');
  const [namespace, setNamespace] = useState('');
  const [kind, setKind] = useState('');

  const tenants = useMemo(() => tenantsOf(resources), [resources]);
  const activeTenant = tenants.includes(tenant) ? tenant : (tenants[0] ?? '');

  // Namespace + kind options are scoped to the active tenant; each selection
  // resets to its default ("all" / "apps") when invalid for the current tenant.
  const namespaces = useMemo(
    () => namespacesOf(resources, activeTenant || undefined),
    [resources, activeTenant],
  );
  const activeNamespace = namespaces.includes(namespace) ? namespace : '';

  const kinds = useMemo(() => kindsOf(resources, activeTenant || undefined), [resources, activeTenant]);
  const activeKind = kinds.includes(kind) ? kind : '';

  const matrix = useMemo(
    () =>
      buildMatrix(resources, {
        tenant: activeTenant || undefined,
        namespace: activeNamespace || undefined,
        kind: activeKind || undefined,
      }),
    [resources, activeTenant, activeNamespace, activeKind],
  );

  const handleSelect = (cell: MatrixCell) => {
    const r = cell.resource ?? cell.children?.[0];
    if (r) onSelectCell({ cluster: r.cluster, kind: r.kind, namespace: r.namespace, name: r.name });
  };

  return (
    <div className="matrix">
      {tenants.length > 1 && (
        <div className="matrix__tenants">
          <span className="matrix__tenants-label">Tenant</span>
          <Tabs value={activeTenant} onChange={setTenant} data-size="sm">
            <Tabs.List>
              {tenants.map((t) => (
                <Tabs.Tab key={t} value={t}>
                  {t}
                </Tabs.Tab>
              ))}
            </Tabs.List>
          </Tabs>
        </div>
      )}

      <div className="matrix__filters">
        <Field>
          <Label data-size="sm">Namespace</Label>
          <Select
            value={activeNamespace}
            onChange={(e) => setNamespace(e.target.value)}
            data-size="sm"
            width="auto"
          >
            <Select.Option value="">All namespaces</Select.Option>
            {namespaces.map((ns) => (
              <Select.Option key={ns} value={ns}>
                {ns}
              </Select.Option>
            ))}
          </Select>
        </Field>
        <Field>
          <Label data-size="sm">Kind</Label>
          <Select value={activeKind} onChange={(e) => setKind(e.target.value)} data-size="sm" width="auto">
            <Select.Option value="">Apps</Select.Option>
            {kinds.map((k) => (
              <Select.Option key={k} value={k}>
                {k}
              </Select.Option>
            ))}
          </Select>
        </Field>
        <span className="matrix__count">
          {activeKind
            ? `${matrix.rows.length} ${activeKind}`
            : `${matrix.rows.length} app${matrix.rows.length === 1 ? '' : 's'}`}
        </span>
      </div>

      <div className="matrix__scroll">
        <Table stickyHeader hover data-size="sm">
          <caption className="sr-only">Deployment status of each app across environments</caption>
          <Table.Head>
            <Table.Row>
              <Table.HeaderCell scope="col">App</Table.HeaderCell>
              {matrix.envs.map((env) => (
                <Table.HeaderCell key={env} scope="col">
                  {envLabel(env)}
                </Table.HeaderCell>
              ))}
            </Table.Row>
          </Table.Head>
          <Table.Body>
            {matrix.rows.length === 0 ? (
              <Table.Row>
                <Table.Cell colSpan={matrix.envs.length + 1}>
                  No deployments match the current filter.
                </Table.Cell>
              </Table.Row>
            ) : (
              matrix.rows.map((row) => (
                <Table.Row key={row.key}>
                  <Table.HeaderCell scope="row">
                    <span className="matrix__app">
                      <span className="matrix__app-name">
                        <strong>{row.name}</strong>
                        {row.drift && (
                          <Tag data-color="warning" data-size="sm" variant="outline">
                            drift
                          </Tag>
                        )}
                      </span>
                      <span className="matrix__app-meta">
                        {row.kind} · {row.namespace}
                      </span>
                    </span>
                  </Table.HeaderCell>
                  {matrix.envs.map((env) => (
                    <Table.Cell key={env}>
                      <StatusCell cell={row.cells[env]} env={env} appName={row.name} onSelect={handleSelect} />
                    </Table.Cell>
                  ))}
                </Table.Row>
              ))
            )}
          </Table.Body>
        </Table>
      </div>
    </div>
  );
}
