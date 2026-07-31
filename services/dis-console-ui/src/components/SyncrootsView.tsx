import { useMemo, useState } from 'react';
import {
  Button,
  Field,
  Heading,
  Label,
  Link,
  Paragraph,
  Select,
  Table,
  Tabs,
  Tag,
} from '@digdir/designsystemet-react';
import { ArrowLeft } from 'lucide-react';
import type { Artifact, Resource } from '../api/types';
import type { ResourceRef } from '../hooks/useResourceDetail';
import { useHashRoute } from '../hooks/useHashRoute';
import {
  artifactStatus,
  artifactTag,
  artifactTenantsOf,
  buildArtifactMatrix,
  deployedBySyncroot,
  inFlight,
  shortDigest,
  syncrootArtifacts,
  type ArtifactRow,
} from '../lib/artifacts';
import { portalUrl } from '../lib/azure';
import {
  DIS_PRODUCTS,
  ENV_ORDER,
  STATUS_SEVERITY,
  envLabel,
  envOf,
  isApp,
  shortRev,
  statusOf,
  tenantOf,
  type DeployStatus,
  type Environment,
} from '../lib/flux';
import { buildMatrix, worstOf } from '../lib/matrix';
import { buildSyncrootReleases } from '../lib/releases';
import { linkFromOrigin } from '../lib/sourceLink';
import { statusStyle } from '../lib/statusColor';
import { sortRows, toggleSort, type SortCol, type SortState } from '../lib/tableSort';
import { syncrootWorkloads, type AppWorkloadRow } from '../lib/workloads';
import { KustomizationPage } from './KustomizationPage';
import { ReleasesBrowser } from './ReleasesBrowser';
import { RouteLink } from './RouteLink';
import { SortableTh } from './SortableTh';
import { WorkloadsTable } from './WorkloadsTable';
import { StageChip } from './StageChip';
import { StatusTag } from './StatusTag';
import { SyncrootMap } from './SyncrootMap';

const PORTAL_TENANT = import.meta.env.VITE_AZURE_PORTAL_TENANT || undefined;
const GRAFANA = (import.meta.env.VITE_GRAFANA_BASE_URL || '').replace(/\/$/, '');

interface Props {
  artifacts: Artifact[];
  resources: Resource[];
  onSelectResource: (ref: ResourceRef) => void;
  onSelectArtifact: (artifact: Artifact) => void;
}

/** The Syncroots section: a list of the fleet's syncroots (with their per-env
 *  digests) → a project-style page per syncroot → a page per Kustomization.
 *  Pages are hash-routed, so the browser's back button and deep links work. */
export function SyncrootsView({ artifacts, resources, onSelectResource, onSelectArtifact }: Props) {
  const [tenant, setTenant] = useState('');
  const [route, navigate] = useHashRoute();

  const syncroots = useMemo(() => syncrootArtifacts(artifacts), [artifacts]);
  const tenants = useMemo(() => artifactTenantsOf(syncroots), [syncroots]);
  const activeTenant = tenants.includes(tenant) ? tenant : (tenants[0] ?? '');

  const matrix = useMemo(
    () => buildArtifactMatrix(syncroots, { tenant: activeTenant || undefined }),
    [syncroots, activeTenant],
  );
  // Unfiltered lookup for routed pages — a deep link must resolve regardless
  // of which tenant tab the list happens to be on.
  const allRows = useMemo(() => buildArtifactMatrix(syncroots), [syncroots]);

  if (route.view === 'kustomization') {
    return (
      <KustomizationPage
        kust={route}
        resources={resources}
        onBack={() =>
          window.history.length > 1 ? window.history.back() : navigate({ view: 'syncroots' })
        }
        onSelectResource={onSelectResource}
      />
    );
  }

  if (route.view === 'syncroot') {
    const row = allRows.rows.find((r) => r.key === route.key);
    if (row) {
      return (
        <SyncrootPage
          row={row}
          resources={resources}
          env={route.env ?? ''}
          tab={route.tab ?? ''}
          onEnv={(e) =>
            navigate({ view: 'syncroot', key: row.key, env: e, tab: route.tab }, { replace: true })
          }
          onTab={(t, e) => navigate({ view: 'syncroot', key: row.key, env: e, tab: t })}
          onBack={() => navigate({ view: 'syncroots' })}
          onSelectResource={onSelectResource}
          onSelectArtifact={onSelectArtifact}
        />
      );
    }
  }

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

      <span className="matrix__count">
        {matrix.rows.length} syncroot{matrix.rows.length === 1 ? '' : 's'} — click one to see what
        it deploys
      </span>

      <div className="matrix__scroll">
        <Table stickyHeader hover data-size="sm">
          <caption className="sr-only">Syncroots and their artifact digests per environment</caption>
          <Table.Head>
            <Table.Row>
              <Table.HeaderCell scope="col">Syncroot</Table.HeaderCell>
              {matrix.envs.map((e) => (
                <Table.HeaderCell key={e} scope="col">
                  {envLabel(e)}
                </Table.HeaderCell>
              ))}
            </Table.Row>
          </Table.Head>
          <Table.Body>
            {matrix.rows.length === 0 ? (
              <Table.Row>
                <Table.Cell colSpan={matrix.envs.length + 1}>No syncroots reported.</Table.Cell>
              </Table.Row>
            ) : (
              matrix.rows.map((row) => (
                <Table.Row key={row.key}>
                  <Table.HeaderCell scope="row">
                    <span className="matrix__app">
                      <span className="matrix__app-name">
                        <RouteLink route={{ view: 'syncroot', key: row.key }}>
                          <strong>{row.owner || row.name}</strong>
                        </RouteLink>
                        <Tag data-color="neutral" data-size="sm" variant="outline">
                          {row.class === 'admin-syncroot' ? 'admin' : 'product'}
                        </Tag>
                      </span>
                      <span className="matrix__app-meta">
                        {row.name} · {row.namespace}
                      </span>
                    </span>
                  </Table.HeaderCell>
                  {matrix.envs.map((e) => {
                    const cell = row.cells[e];
                    return (
                      <Table.Cell key={e}>
                        {cell ? (
                          <RouteLink
                            route={{ view: 'syncroot', key: row.key, env: e }}
                            className="cell-button"
                          >
                            <span className="cell">
                              <Tag
                                data-color={statusStyle(artifactStatus(cell.artifact)).color}
                                data-size="sm"
                                variant={statusStyle(artifactStatus(cell.artifact)).variant}
                              >
                                {shortDigest(cell.artifact.revision) || '—'}
                              </Tag>
                              {cell.inFlight ? <span className="cell__rev">rolling…</span> : null}
                            </span>
                          </RouteLink>
                        ) : (
                          <StatusTag status="absent" />
                        )}
                      </Table.Cell>
                    );
                  })}
                </Table.Row>
              ))
            )}
          </Table.Body>
        </Table>
      </div>
    </div>
  );
}

interface SyncrootRow {
  kind: string;
  namespace: string;
  name: string;
  status: DeployStatus;
  resource: Resource;
}

const CATEGORIES: { label: string; kinds: readonly string[] }[] = [
  { label: 'Apps', kinds: ['Kustomization', 'HelmRelease'] },
  { label: 'Databases', kinds: DIS_PRODUCTS.databases },
  { label: 'Vaults', kinds: DIS_PRODUCTS.vaults },
  { label: 'Identities', kinds: DIS_PRODUCTS.identities },
  { label: 'APIM', kinds: DIS_PRODUCTS.apim },
];

const PAGE_TABS = [
  'overview',
  'resources',
  'workloads',
  'map',
  'releases',
  'access',
  'observability',
];

function SyncrootPage({
  row,
  resources,
  env,
  tab,
  onEnv,
  onTab,
  onBack,
  onSelectResource,
  onSelectArtifact,
}: {
  row: ArtifactRow;
  resources: Resource[];
  env: string;
  tab: string;
  onEnv: (env: string) => void;
  /** Tab changes are page navigation: pushed to history so Back walks tabs. */
  onTab: (tab: string, env: string) => void;
  onBack: () => void;
  onSelectResource: (ref: ResourceRef) => void;
  onSelectArtifact: (artifact: Artifact) => void;
}) {
  const [kind, setKind] = useState('');
  const [status, setStatus] = useState('');
  const [ns, setNs] = useState('');
  const [sort, setSort] = useState<SortState>({ col: 'kind', dir: 'asc' });
  const activeTab = PAGE_TABS.includes(tab) ? tab : 'overview';
  const envs = Object.keys(row.cells);
  const orderedEnvs = useMemo(
    () => [
      ...ENV_ORDER.filter((e) => envs.includes(e)),
      ...envs.filter((e) => !ENV_ORDER.includes(e as Environment)).sort(),
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [row],
  );
  const activeEnv = envs.includes(env) ? env : (orderedEnvs[0] ?? '');
  const cell = row.cells[activeEnv];
  const artifact = cell?.artifact;

  const deployed: SyncrootRow[] = useMemo(
    () =>
      artifact
        ? deployedBySyncroot(resources, artifact.cluster, artifact.kustomizations).map((r) => ({
            kind: r.kind,
            namespace: r.namespace,
            name: r.name,
            status: statusOf(r),
            resource: r,
          }))
        : [],
    [resources, artifact],
  );
  const kinds = useMemo(() => [...new Set(deployed.map((r) => r.kind))].sort(), [deployed]);
  const activeKind = kinds.includes(kind) ? kind : '';
  const statuses = useMemo(
    () =>
      [...new Set(deployed.map((r) => r.status))].sort(
        (a, b) => STATUS_SEVERITY[b] - STATUS_SEVERITY[a],
      ),
    [deployed],
  );
  const activeStatus = statuses.includes(status as DeployStatus) ? status : '';
  const namespaces = useMemo(() => [...new Set(deployed.map((r) => r.namespace))].sort(), [deployed]);
  const activeNs = namespaces.includes(ns) ? ns : '';
  const shown = useMemo(
    () =>
      sortRows(
        deployed.filter(
          (r) =>
            (!activeKind || r.kind === activeKind) &&
            (!activeStatus || r.status === activeStatus) &&
            (!activeNs || r.namespace === activeNs),
        ),
        sort,
      ),
    [deployed, activeKind, activeStatus, activeNs, sort],
  );
  const onSort = (col: SortCol) => setSort((s) => toggleSort(s, col));
  const origin = artifact ? linkFromOrigin(artifact.originSource, artifact.originRevision) : null;
  const releases = useMemo(() => buildSyncrootReleases(row, orderedEnvs), [row, orderedEnvs]);

  // The syncroot's apps as app × environment rows for the releases browser:
  // the union of the appliedBy closure across every environment, intersected
  // with the tenant's deployment matrix — the matrix decides what counts as a
  // row (apps, plus root-applied workloads; folded children ride their row).
  const appRows = useMemo(() => {
    if (!artifact) return [];
    const ids = new Set<string>();
    for (const e of orderedEnvs) {
      const c = row.cells[e];
      if (!c) continue;
      for (const r of deployedBySyncroot(resources, c.artifact.cluster, c.artifact.kustomizations)) {
        ids.add(`${r.kind}|${r.namespace}|${r.name}`);
      }
    }
    return buildMatrix(resources, { tenant: tenantOf(artifact.cluster) }).rows.filter((r) =>
      ids.has(r.key),
    );
  }, [resources, row, orderedEnvs, artifact]);

  // Every workload this syncroot deploys — read from the appliedBy closure,
  // so workloads applied directly by the root Kustomization (no app of their
  // own) appear too. Empty until the fleet API's schema-v5 workload sweep is
  // rolled out.
  const workloads: AppWorkloadRow[] = useMemo(
    () => syncrootWorkloads(resources, row, orderedEnvs),
    [resources, row, orderedEnvs],
  );

  const categories = useMemo(
    () =>
      CATEGORIES.map((c) => {
        const rs = deployed.filter((r) => c.kinds.includes(r.kind));
        return {
          ...c,
          count: rs.length,
          worst: rs.reduce<DeployStatus>((w, r) => worstOf(w, r.status), 'absent'),
        };
      }).filter((c) => c.count > 0),
    [deployed],
  );
  const identities = useMemo(
    () => deployed.filter((r) => r.kind === 'ApplicationIdentity'),
    [deployed],
  );
  const apps = useMemo(() => deployed.filter((r) => isApp(r.resource)), [deployed]);

  const resourceName = (r: SyncrootRow, openKustomizations: boolean) =>
    openKustomizations && r.kind === 'Kustomization' ? (
      <RouteLink
        route={{
          view: 'kustomization',
          cluster: r.resource.cluster,
          namespace: r.namespace,
          name: r.name,
        }}
      >
        {r.name}
      </RouteLink>
    ) : (
      <button
        type="button"
        className="tree__name"
        onClick={() =>
          onSelectResource({
            cluster: r.resource.cluster,
            kind: r.kind,
            namespace: r.namespace,
            name: r.name,
          })
        }
      >
        {r.name}
      </button>
    );

  return (
    <div className="syncroots">
      <div className="syncroot-page__head">
        <Button variant="tertiary" data-size="sm" onClick={onBack}>
          <ArrowLeft size="1em" aria-hidden /> Syncroots
        </Button>
        <Heading level={2} data-size="sm">
          {row.owner || row.name}
        </Heading>
        <Tag data-color="neutral" data-size="sm" variant="outline">
          {row.class === 'admin-syncroot' ? 'admin syncroot' : 'product syncroot'}
        </Tag>
        <Field className="syncroot-page__env">
          <Label data-size="sm" className="sr-only">
            Environment
          </Label>
          <Select value={activeEnv} onChange={(e) => onEnv(e.target.value)} data-size="sm" width="auto">
            {orderedEnvs.map((e) => (
              <Select.Option key={e} value={e}>
                {envLabel(e)}
              </Select.Option>
            ))}
          </Select>
        </Field>
      </div>

      {!artifact ? (
        <Paragraph data-color="neutral">This syncroot is absent from {envLabel(activeEnv)}.</Paragraph>
      ) : (
        <Tabs
          value={activeTab}
          onChange={(v) => {
            if (v !== activeTab) onTab(v, activeEnv);
          }}
        >
          <Tabs.List>
            <Tabs.Tab value="overview">Overview</Tabs.Tab>
            <Tabs.Tab value="resources">Resources</Tabs.Tab>
            <Tabs.Tab value="workloads">Workloads</Tabs.Tab>
            <Tabs.Tab value="map">Map</Tabs.Tab>
            <Tabs.Tab value="releases">Releases</Tabs.Tab>
            <Tabs.Tab value="access">Access</Tabs.Tab>
            <Tabs.Tab value="observability">Observability</Tabs.Tab>
          </Tabs.List>

          <Tabs.Panel value="overview">
            <div className="overview">
              <section className="overview__card">
                <Heading level={3} data-size="2xs">
                  Status
                </Heading>
                <div className="overview__row">
                  <StatusTag status={artifactStatus(artifact)} />
                  {inFlight(artifact) && <span className="cell__rev">rolling…</span>}
                </div>
                <dl className="overview__facts">
                  <dt>Fetched</dt>
                  <dd>
                    {shortDigest(artifact.revision) || '—'}
                    {artifactTag(artifact.revision) ? ` (tag ${artifactTag(artifact.revision)})` : ''}
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
                  <dt>Deployed by</dt>
                  <dd>
                    {artifact.kustomizations.length === 0
                      ? '—'
                      : artifact.kustomizations.map((k, i) => (
                          <span key={`${k.namespace}/${k.name}`}>
                            {i > 0 ? ', ' : ''}
                            <RouteLink
                              route={{
                                view: 'kustomization',
                                cluster: artifact.cluster,
                                namespace: k.namespace,
                                name: k.name,
                              }}
                            >
                              {k.namespace}/{k.name}
                            </RouteLink>
                          </span>
                        ))}
                  </dd>
                  <dt>Artifact</dt>
                  <dd>
                    <button type="button" className="tree__name" onClick={() => onSelectArtifact(artifact)}>
                      {artifact.name}
                    </button>
                  </dd>
                </dl>
              </section>

              <section className="overview__card">
                <Heading level={3} data-size="2xs">
                  Resources
                </Heading>
                {categories.map((c) => (
                  <div key={c.label} className="overview__row overview__row--split">
                    <span>{c.label}</span>
                    <span className="overview__row">
                      <span className="overview__count">{c.count}</span>
                      <StatusTag status={c.worst} />
                    </span>
                  </div>
                ))}
                <div className="overview__total">
                  {deployed.length} resources in {namespaces.length} namespace
                  {namespaces.length === 1 ? '' : 's'}
                </div>
              </section>

              <section className="overview__card">
                <Heading level={3} data-size="2xs">
                  Namespaces
                </Heading>
                <div className="overview__chips">
                  {namespaces.map((n) => (
                    <Tag key={n} data-color="neutral" data-size="sm" variant="outline">
                      {n}
                    </Tag>
                  ))}
                  {namespaces.length === 0 && <span className="tree__muted">none yet</span>}
                </div>
              </section>
            </div>
          </Tabs.Panel>

          <Tabs.Panel value="resources">
            <div className="syncroots">
              <div className="matrix__filters">
                <Field>
                  <Label data-size="sm">Namespace</Label>
                  <Select value={activeNs} onChange={(e) => setNs(e.target.value)} data-size="sm" width="auto">
                    <Select.Option value="">All namespaces</Select.Option>
                    {namespaces.map((n) => (
                      <Select.Option key={n} value={n}>
                        {n}
                      </Select.Option>
                    ))}
                  </Select>
                </Field>
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
                  {activeKind || activeStatus || activeNs
                    ? `${shown.length} of ${deployed.length} resources`
                    : `${shown.length} resource${shown.length === 1 ? '' : 's'}`}{' '}
                  on {tenantOf(artifact.cluster)} {envLabel(envOf(artifact.cluster))}
                </span>
              </div>

              <div className="matrix__scroll">
                <Table stickyHeader hover data-size="sm">
                  <caption className="sr-only">Resources deployed by this syncroot</caption>
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
                    {shown.length === 0 ? (
                      <Table.Row>
                        <Table.Cell colSpan={5}>
                          No swept resources chain back to this syncroot yet.
                        </Table.Cell>
                      </Table.Row>
                    ) : (
                      shown.map((r) => (
                        <Table.Row key={`${r.kind}/${r.namespace}/${r.name}`}>
                          <Table.Cell>{r.kind}</Table.Cell>
                          <Table.Cell>{r.namespace}</Table.Cell>
                          <Table.Cell>{resourceName(r, true)}</Table.Cell>
                          <Table.Cell>
                            <StatusTag status={r.status} />
                          </Table.Cell>
                          <Table.Cell>
                            <span className="cell__rev">{shortRev(r.resource.revision)}</span>
                          </Table.Cell>
                        </Table.Row>
                      ))
                    )}
                  </Table.Body>
                </Table>
              </div>
            </div>
          </Tabs.Panel>

          <Tabs.Panel value="workloads">
            <div className="syncroots">
              <Paragraph data-size="sm" data-color="neutral">
                Declared container images per workload and environment — the apps' effective
                versions. A yellow tag means the container's image differs across environments.
              </Paragraph>
              {workloads.length === 0 ? (
                <Paragraph data-size="sm" data-color="neutral">
                  No workloads mirrored yet — the fleet API sweeps kustomize-applied workloads
                  from schema v5.
                </Paragraph>
              ) : (
                <WorkloadsTable
                  workloads={workloads}
                  envs={orderedEnvs}
                  caption="Workloads deployed by this syncroot and their container images per environment"
                />
              )}
            </div>
          </Tabs.Panel>

          <Tabs.Panel value="map">
            <div className="syncroots">
              <SyncrootMap
                artifact={artifact}
                resources={resources}
                onSelectResource={onSelectResource}
                onSelectArtifact={onSelectArtifact}
              />
            </div>
          </Tabs.Panel>

          <Tabs.Panel value="releases">
            <div className="syncroots">
              <ReleasesBrowser
                rows={appRows}
                envs={orderedEnvs}
                resources={resources}
                onSelectResource={onSelectResource}
                lead={{
                  key: '__syncroot',
                  label: row.owner || row.name,
                  meta: 'syncroot artifact',
                  content: (
                    <>
                      <Paragraph data-size="sm" data-color="neutral">
                        A release of the syncroot is an artifact digest. Newest first — a chip
                        shows whether an environment runs it, is pulling it, or never had it.
                      </Paragraph>
                      <div className="matrix__scroll">
                        <Table hover data-size="sm">
                          <caption className="sr-only">
                            This syncroot's releases across environments
                          </caption>
                          <Table.Head>
                            <Table.Row>
                              <Table.HeaderCell scope="col">Release</Table.HeaderCell>
                              {orderedEnvs.map((e) => (
                                <Table.HeaderCell key={e} scope="col">
                                  {envLabel(e)}
                                </Table.HeaderCell>
                              ))}
                            </Table.Row>
                          </Table.Head>
                          <Table.Body>
                            {releases.map((rel) => {
                              const relOrigin = linkFromOrigin(rel.originSource, rel.originRevision);
                              return (
                                <Table.Row key={rel.digest}>
                                  <Table.HeaderCell scope="row">
                                    <span className="matrix__app">
                                      <strong>{rel.digest}</strong>
                                      {relOrigin && (
                                        <span className="matrix__app-meta">
                                          <Link
                                            href={relOrigin.commitUrl ?? relOrigin.repoUrl}
                                            target="_blank"
                                            rel="noreferrer"
                                          >
                                            {relOrigin.ref ? `${relOrigin.ref} @ ` : ''}
                                            {relOrigin.shortSha ?? relOrigin.label}
                                          </Link>
                                        </span>
                                      )}
                                    </span>
                                  </Table.HeaderCell>
                                  {orderedEnvs.map((e) => (
                                    <Table.Cell key={e}>
                                      <button
                                        type="button"
                                        className="cell-button"
                                        aria-label={`${rel.digest} in ${envLabel(e)}: ${rel.chips[e] ?? 'absent'}`}
                                        onClick={() => onEnv(e)}
                                      >
                                        <StageChip state={rel.chips[e] ?? 'absent'} label={envLabel(e)} />
                                      </button>
                                    </Table.Cell>
                                  ))}
                                </Table.Row>
                              );
                            })}
                          </Table.Body>
                        </Table>
                      </div>
                    </>
                  ),
                }}
              />
            </div>
          </Tabs.Panel>

          <Tabs.Panel value="access">
            <div className="syncroots">
              <Paragraph data-size="sm" data-color="neutral">
                Cluster access is GitOps-only — changes ship through this syncroot's artifact.
                Workloads run under the managed identities below.
              </Paragraph>
              <div className="matrix__scroll">
                <Table hover data-size="sm">
                  <caption className="sr-only">Managed identities deployed by this syncroot</caption>
                  <Table.Head>
                    <Table.Row>
                      <Table.HeaderCell scope="col">Identity</Table.HeaderCell>
                      <Table.HeaderCell scope="col">Namespace</Table.HeaderCell>
                      <Table.HeaderCell scope="col">Status</Table.HeaderCell>
                      <Table.HeaderCell scope="col">Azure</Table.HeaderCell>
                    </Table.Row>
                  </Table.Head>
                  <Table.Body>
                    {identities.length === 0 ? (
                      <Table.Row>
                        <Table.Cell colSpan={4}>No ApplicationIdentities in this syncroot.</Table.Cell>
                      </Table.Row>
                    ) : (
                      identities.map((r) => (
                        <Table.Row key={`${r.namespace}/${r.name}`}>
                          <Table.Cell>{resourceName(r, false)}</Table.Cell>
                          <Table.Cell>{r.namespace}</Table.Cell>
                          <Table.Cell>
                            <StatusTag status={r.status} />
                          </Table.Cell>
                          <Table.Cell>
                            {r.resource.azureResourceId ? (
                              <Link
                                href={portalUrl(r.resource.azureResourceId, PORTAL_TENANT)}
                                target="_blank"
                                rel="noreferrer"
                              >
                                Azure Portal ↗
                              </Link>
                            ) : (
                              <span className="tree__muted">—</span>
                            )}
                          </Table.Cell>
                        </Table.Row>
                      ))
                    )}
                  </Table.Body>
                </Table>
              </div>
            </div>
          </Tabs.Panel>

          <Tabs.Panel value="observability">
            <div className="syncroots">
              {!GRAFANA && (
                <Paragraph data-size="sm" data-color="neutral">
                  Set <code>VITE_GRAFANA_BASE_URL</code> to link logs and metrics to the shared
                  Grafana.
                </Paragraph>
              )}
              <div className="matrix__scroll">
                <Table hover data-size="sm">
                  <caption className="sr-only">Apps and their observability links</caption>
                  <Table.Head>
                    <Table.Row>
                      <Table.HeaderCell scope="col">App</Table.HeaderCell>
                      <Table.HeaderCell scope="col">Namespace</Table.HeaderCell>
                      <Table.HeaderCell scope="col">Status</Table.HeaderCell>
                      <Table.HeaderCell scope="col">Grafana</Table.HeaderCell>
                    </Table.Row>
                  </Table.Head>
                  <Table.Body>
                    {apps.length === 0 ? (
                      <Table.Row>
                        <Table.Cell colSpan={4}>No apps in this syncroot.</Table.Cell>
                      </Table.Row>
                    ) : (
                      apps.map((r) => (
                        <Table.Row key={`${r.kind}/${r.namespace}/${r.name}`}>
                          <Table.Cell>{resourceName(r, true)}</Table.Cell>
                          <Table.Cell>{r.namespace}</Table.Cell>
                          <Table.Cell>
                            <StatusTag status={r.status} />
                          </Table.Cell>
                          <Table.Cell>
                            {GRAFANA ? (
                              <span className="overview__row">
                                <Link
                                  href={`${GRAFANA}/explore?var-cluster=${encodeURIComponent(r.resource.cluster)}&var-namespace=${encodeURIComponent(r.namespace)}`}
                                  target="_blank"
                                  rel="noreferrer"
                                >
                                  Logs ↗
                                </Link>
                                <Link
                                  href={`${GRAFANA}/dashboards?var-cluster=${encodeURIComponent(r.resource.cluster)}&var-namespace=${encodeURIComponent(r.namespace)}`}
                                  target="_blank"
                                  rel="noreferrer"
                                >
                                  Metrics ↗
                                </Link>
                              </span>
                            ) : (
                              <span className="tree__muted">—</span>
                            )}
                          </Table.Cell>
                        </Table.Row>
                      ))
                    )}
                  </Table.Body>
                </Table>
              </div>
            </div>
          </Tabs.Panel>
        </Tabs>
      )}
    </div>
  );
}
