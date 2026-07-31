import { useMemo } from 'react';
import { Spinner, Table, Tabs, Tag } from '@digdir/designsystemet-react';
import type { Cluster, Resource } from '../api/types';
import { useArtifacts } from '../hooks/useArtifacts';
import { syncrootSummaries } from '../lib/artifacts';
import { envLabel } from '../lib/flux';
import { ClustersTable } from './ClustersTable';
import { RouteLink } from './RouteLink';
import { StatusTag } from './StatusTag';

interface Props {
  clusters: Cluster[];
  resources: Resource[];
}

/** The landing page: two flat lists — the fleet's clusters
 *  and its syncroots ("projects" — each groups the namespaces it deploys
 *  into). Tables, not cards: they hold up at fleet scale. */
export function HomeView({ clusters, resources }: Props) {
  const { artifacts, loading } = useArtifacts();
  const summaries = useMemo(() => syncrootSummaries(artifacts, resources), [artifacts, resources]);

  return (
    <Tabs defaultValue="syncroots">
      <Tabs.List>
        <Tabs.Tab value="syncroots">Syncroots</Tabs.Tab>
        <Tabs.Tab value="clusters">Clusters</Tabs.Tab>
      </Tabs.List>
      <Tabs.Panel value="syncroots">
        {loading ? (
          <Spinner aria-label="Loading syncroots" data-size="sm" />
        ) : (
          <div className="matrix__scroll">
            <Table hover data-size="sm">
              <caption className="sr-only">Syncroots and the namespaces they deploy into</caption>
              <Table.Head>
                <Table.Row>
                  <Table.HeaderCell scope="col">Syncroot</Table.HeaderCell>
                  <Table.HeaderCell scope="col">Class</Table.HeaderCell>
                  <Table.HeaderCell scope="col">Namespaces</Table.HeaderCell>
                  <Table.HeaderCell scope="col">Environments</Table.HeaderCell>
                  <Table.HeaderCell scope="col">Status</Table.HeaderCell>
                </Table.Row>
              </Table.Head>
              <Table.Body>
                {summaries.length === 0 ? (
                  <Table.Row>
                    <Table.Cell colSpan={5}>No syncroot artifacts reported.</Table.Cell>
                  </Table.Row>
                ) : (
                  summaries.map((s) => (
                    <Table.Row key={s.key}>
                      <Table.HeaderCell scope="row">
                        <span className="matrix__app">
                          <RouteLink route={{ view: 'syncroot', key: s.key }}>
                            <strong>{s.owner || s.name}</strong>
                          </RouteLink>
                          <span className="matrix__app-meta">
                            {s.name} · {s.namespace}
                          </span>
                        </span>
                      </Table.HeaderCell>
                      <Table.Cell>
                        <Tag data-color="neutral" data-size="sm" variant="outline">
                          {s.class === 'admin-syncroot' ? 'admin' : 'product'}
                        </Tag>
                      </Table.Cell>
                      <Table.Cell>{s.namespaces.length}</Table.Cell>
                      <Table.Cell>{s.envs.map((e) => envLabel(e)).join(', ') || '—'}</Table.Cell>
                      <Table.Cell>
                        <span className="home__card-tags">
                          <StatusTag status={s.worst} />
                          {s.rolling && <span className="cell__rev">rolling…</span>}
                        </span>
                      </Table.Cell>
                    </Table.Row>
                  ))
                )}
              </Table.Body>
            </Table>
          </div>
        )}
      </Tabs.Panel>
      <Tabs.Panel value="clusters">
        <ClustersTable clusters={clusters} />
      </Tabs.Panel>
    </Tabs>
  );
}
