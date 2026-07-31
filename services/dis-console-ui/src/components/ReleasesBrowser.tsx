import { useMemo, useState, type ReactNode } from 'react';
import { Heading, Link, Paragraph, Table, Tabs, Textfield } from '@digdir/designsystemet-react';
import { GitCommitHorizontal } from 'lucide-react';
import type { Resource } from '../api/types';
import { buildAppReleases, ownerRevisionFor, type AppRelease } from '../lib/appReleases';
import { envLabel, isWorkloadKind, type DeployStatus } from '../lib/flux';
import { buildMatrix, tenantsOf, worstOf, type MatrixRow } from '../lib/matrix';
import { releaseCommitUrl } from '../lib/sourceLink';
import { imageLabel, primaryImage, selfWorkload, workloadsOf } from '../lib/workloads';
import { useAppHistories } from '../hooks/useAppHistories';
import type { ResourceRef } from '../hooks/useResourceDetail';
import { ReleaseDialog, type ReleaseSelection } from './ReleaseDialog';
import { StageChip } from './StageChip';

/** An extra first entry in the app list (e.g. the syncroot artifact itself). */
export interface ReleasesLead {
  key: string;
  label: string;
  meta?: string;
  content: ReactNode;
}

function worstStatus(row: MatrixRow): DeployStatus {
  return Object.values(row.cells).reduce<DeployStatus>((w, c) => worstOf(w, c.status), 'absent');
}

function firstSeenLabel(iso?: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? '—'
    : d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

function AppReleasesPanel({
  row,
  envs,
  resources,
  onSelectResource,
}: {
  row: MatrixRow;
  envs: string[];
  resources: Resource[];
  onSelectResource?: (ref: ResourceRef) => void;
}) {
  const { histories, owners, ownerHistories, loading } = useAppHistories(row);
  const current = useMemo(() => {
    const c: Partial<Record<string, Resource>> = {};
    for (const env of envs) {
      const r = row.cells[env]?.resource;
      if (!r) continue;
      // A workload app's release identity is its primary image tag. Until the
      // fleet API projects that as the workload's revision, derive it here so
      // release rows exist (history accrues once the backend ships it).
      if (!r.revision && isWorkloadKind(r.kind)) {
        const img = primaryImage(r.images, r.name);
        c[env] = img ? { ...r, revision: imageLabel(img) } : r;
      } else {
        c[env] = r;
      }
    }
    return c;
  }, [row, envs]);
  const releases = useMemo(
    () => (histories ? buildAppReleases(current, histories, envs) : []),
    [current, histories, envs],
  );
  const [selRelease, setSelRelease] = useState<AppRelease | null>(null);

  // The app's Flux sources across environments (via sourceRef) — their origin
  // annotations give the git repo, and per-release commit links: a git-sha
  // revision links directly, an OCI digest only while a source still fetches
  // that digest.
  const sources = useMemo(() => {
    const out: Resource[] = [];
    for (const env of envs) {
      const r = row.cells[env]?.resource;
      const ref = r?.sourceRef;
      if (!r || !ref) continue;
      const src = resources.find(
        (s) =>
          s.cluster === r.cluster &&
          s.kind === ref.kind &&
          s.name === ref.name &&
          s.namespace === (ref.namespace ?? r.namespace),
      );
      if (src) out.push(src);
    }
    return out;
  }, [row, envs, resources]);
  const repoUrl = useMemo(() => {
    for (const env of envs) {
      const r = row.cells[env]?.resource;
      if (r?.originSource) return r.originSource;
    }
    return sources.find((s) => s.originSource)?.originSource;
  }, [row, envs, sources]);

  // The owning Kustomization's source chain — the HelmRelease fallback: a
  // chart version carries no commit of its own, but the gitops commit that
  // declared it (the owner's revision at the time) does.
  const ownerSources = useMemo(() => {
    const out: Resource[] = [];
    for (const env of envs) {
      const o = owners[env];
      const ref = o?.sourceRef;
      if (!o || !ref) continue;
      const src = resources.find(
        (s) =>
          s.cluster === o.cluster &&
          s.kind === ref.kind &&
          s.name === ref.name &&
          s.namespace === (ref.namespace ?? o.namespace),
      );
      if (src) out.push(src);
    }
    return out;
  }, [owners, envs, resources]);
  const ownerRepoUrl = useMemo(
    () =>
      envs.map((e) => owners[e]?.originSource).find(Boolean) ??
      ownerSources.find((s) => s.originSource)?.originSource,
    [owners, envs, ownerSources],
  );
  const commitFor = (rel: AppRelease): string | null => {
    const direct = releaseCommitUrl(rel.revision, repoUrl, sources);
    if (direct) return direct;
    const ownerRev = ownerRevisionFor(
      rel.revision,
      envs,
      current,
      owners,
      histories ?? {},
      ownerHistories,
    );
    return releaseCommitUrl(ownerRev, ownerRepoUrl, ownerSources);
  };

  // HelmReleases folded into this row: their chart workloads carry the HR as
  // applier, so they count as this app's workloads too.
  const foldedOwners = useMemo(() => {
    const out = new Map<string, { name: string; namespace: string }>();
    for (const cell of Object.values(row.cells)) {
      for (const c of cell?.children ?? []) {
        out.set(`${c.namespace}|${c.name}`, { name: c.name, namespace: c.namespace });
      }
    }
    return [...out.values()];
  }, [row]);

  // Who deploys this app — the parent, front and center. The applier's kind
  // comes from whichever app exists in the sweep under that identity.
  const applier = useMemo(() => {
    for (const env of envs) {
      const r = row.cells[env]?.resource;
      const a = r?.appliedBy;
      if (!r || !a) continue;
      const find = (kind: string) =>
        resources.find(
          (s) =>
            s.cluster === r.cluster &&
            s.kind === kind &&
            s.namespace === a.namespace &&
            s.name === a.name,
        );
      const target = find('Kustomization') ?? find('HelmRelease');
      return target
        ? {
            kind: target.kind,
            namespace: target.namespace,
            name: target.name,
            cluster: target.cluster,
            swept: true,
          }
        : { kind: '', namespace: a.namespace, name: a.name, cluster: r.cluster, swept: false };
    }
    return null;
  }, [row, envs, resources]);

  // The drawer for one clicked release: its workloads are scoped to the
  // environments running it right now (declared images are only known for
  // running releases), with drift recomputed within that scope.
  const selection: ReleaseSelection | null = selRelease
    ? (() => {
        const runningEnvs = envs.filter((e) => current[e]?.revision === selRelease.revision);
        return {
          app: { name: row.name, namespace: row.namespace, kind: row.kind },
          applierLabel: applier
            ? `${applier.kind ? `${applier.kind} ` : ''}${applier.namespace}/${applier.name}`
            : undefined,
          release: selRelease,
          commitUrl: commitFor(selRelease),
          envs,
          runningEnvs,
          workloads: isWorkloadKind(row.kind)
            ? selfWorkload(current, runningEnvs)
            : workloadsOf(resources, row, runningEnvs, foldedOwners),
        };
      })()
    : null;

  return (
    <section className="relbrowser__detail" aria-label={`Releases of ${row.name}`}>
      <header className="relbrowser__detail-head">
        <Heading level={3} data-size="xs">
          {row.name}
        </Heading>
        <span className="relbrowser__meta">
          {row.namespace} · {row.kind}
        </span>
        {applier && (
          <span className="relbrowser__meta">
            deployed by{' '}
            {onSelectResource && applier.swept ? (
              <button
                type="button"
                className="tree__name"
                onClick={() =>
                  onSelectResource({
                    cluster: applier.cluster,
                    kind: applier.kind,
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
          </span>
        )}
      </header>
      {loading ? (
        <Paragraph data-size="sm" data-color="neutral">
          Loading release history…
        </Paragraph>
      ) : (
        <>
          <div className="matrix__scroll">
            <Table hover data-size="sm">
              <caption className="sr-only">Releases of {row.name} across environments</caption>
              <Table.Head>
                <Table.Row>
                  <Table.HeaderCell scope="col">Release</Table.HeaderCell>
                  <Table.HeaderCell scope="col">First seen</Table.HeaderCell>
                  {envs.map((e) => (
                    <Table.HeaderCell key={e} scope="col">
                      {envLabel(e)}
                    </Table.HeaderCell>
                  ))}
                </Table.Row>
              </Table.Head>
              <Table.Body>
                {releases.length === 0 ? (
                  <Table.Row>
                    <Table.Cell colSpan={envs.length + 2}>No releases recorded.</Table.Cell>
                  </Table.Row>
                ) : (
                  releases.map((rel) => {
                    const commit = commitFor(rel);
                    return (
                    <Table.Row key={rel.revision}>
                      <Table.HeaderCell scope="row">
                        <span className="release-cell">
                          <button
                            type="button"
                            className="tree__name"
                            onClick={() => setSelRelease(rel)}
                          >
                            <strong>{rel.shortRev}</strong>
                          </button>
                          {commit && (
                            <Link
                              href={commit}
                              target="_blank"
                              rel="noreferrer"
                              className="release-cell__commit"
                              aria-label={`View the commit behind ${rel.shortRev}`}
                              title="View the commit behind this release"
                            >
                              <GitCommitHorizontal aria-hidden size="1.05rem" />
                            </Link>
                          )}
                        </span>
                      </Table.HeaderCell>
                      <Table.Cell>{firstSeenLabel(rel.firstSeen)}</Table.Cell>
                      {envs.map((e) => (
                        <Table.Cell key={e}>
                          <StageChip state={rel.chips[e] ?? 'absent'} label={envLabel(e)} />
                        </Table.Cell>
                      ))}
                    </Table.Row>
                    );
                  })
                )}
              </Table.Body>
            </Table>
          </div>
          <Paragraph data-size="xs" data-color="neutral">
            Built from the fleet API's status-event history — releases older than the server's
            retention window age out. Click a release for its workloads and commit.
          </Paragraph>
        </>
      )}
      <ReleaseDialog selected={selection} onClose={() => setSelRelease(null)} />
    </section>
  );
}

/**
 * The releases browser: a searchable app list on the left, the
 * selected app's releases (revision × environment stage chips) on the right.
 */
export function ReleasesBrowser({
  rows,
  envs,
  resources,
  lead,
  onSelectResource,
}: {
  rows: MatrixRow[];
  envs: string[];
  /** The full resource list — used to resolve apps' sources for commit links. */
  resources: Resource[];
  lead?: ReleasesLead;
  /** Opens the resource drawer (the applier link in the panel header). */
  onSelectResource?: (ref: ResourceRef) => void;
}) {
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<string | null>(null);

  const q = query.trim().toLowerCase();
  const shown = useMemo(
    () =>
      rows.filter(
        (r) =>
          !q ||
          r.name.toLowerCase().includes(q) ||
          r.namespace.toLowerCase().includes(q) ||
          r.kind.toLowerCase().includes(q),
      ),
    [rows, q],
  );
  const valid = selected && (selected === lead?.key || rows.some((r) => r.key === selected));
  const activeKey = valid ? selected : (lead?.key ?? rows[0]?.key ?? null);
  const activeRow = rows.find((r) => r.key === activeKey);

  if (!lead && rows.length === 0) {
    return (
      <Paragraph data-size="sm" data-color="neutral">
        No apps to show releases for.
      </Paragraph>
    );
  }

  return (
    <div className="relbrowser">
      <aside className="relbrowser__list" aria-label="Apps">
        <Textfield
          data-size="sm"
          aria-label="Search apps"
          placeholder="Search apps"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <div className="relbrowser__items">
          {lead && (
            <button
              type="button"
              className="relbrowser__item"
              aria-current={activeKey === lead.key}
              onClick={() => setSelected(lead.key)}
            >
              <span className="relbrowser__item-name">{lead.label}</span>
              {lead.meta && <span className="relbrowser__meta">{lead.meta}</span>}
            </button>
          )}
          {shown.map((r) => (
            <button
              key={r.key}
              type="button"
              className="relbrowser__item"
              aria-current={activeKey === r.key}
              onClick={() => setSelected(r.key)}
            >
              <span className="relbrowser__item-name">
                <span className="relbrowser__dot" data-status={worstStatus(r)} aria-hidden />
                {r.name}
              </span>
              <span className="relbrowser__meta">
                {r.namespace} · {r.kind}
              </span>
            </button>
          ))}
          {shown.length === 0 && (
            <Paragraph data-size="sm" data-color="neutral" className="relbrowser__empty">
              No apps match “{query}”.
            </Paragraph>
          )}
        </div>
      </aside>
      {lead && activeKey === lead.key ? (
        <section className="relbrowser__detail" aria-label={`Releases of ${lead.label}`}>
          <header className="relbrowser__detail-head">
            <Heading level={3} data-size="xs">
              {lead.label}
            </Heading>
            {lead.meta && <span className="relbrowser__meta">{lead.meta}</span>}
          </header>
          {lead.content}
        </section>
      ) : activeRow ? (
        <AppReleasesPanel
          key={activeRow.key}
          row={activeRow}
          envs={envs}
          resources={resources}
          onSelectResource={onSelectResource}
        />
      ) : null}
    </div>
  );
}

/**
 * The Deployments-level releases overview: tenant tabs over every app the
 * fleet API knows, each opening its releases view.
 */
export function ReleasesOverview({
  resources,
  onSelectResource,
}: {
  resources: Resource[];
  onSelectResource?: (ref: ResourceRef) => void;
}) {
  const tenants = useMemo(() => tenantsOf(resources), [resources]);
  const [tenant, setTenant] = useState('');
  const activeTenant = tenants.includes(tenant) ? tenant : (tenants[0] ?? '');
  const matrix = useMemo(
    () => buildMatrix(resources, { tenant: activeTenant }),
    [resources, activeTenant],
  );

  return (
    <div className="relbrowser__overview">
      {tenants.length > 1 && (
        <Tabs value={activeTenant} onChange={setTenant} data-size="sm">
          <Tabs.List>
            {tenants.map((t) => (
              <Tabs.Tab key={t} value={t}>
                {t.toUpperCase()}
              </Tabs.Tab>
            ))}
          </Tabs.List>
        </Tabs>
      )}
      <ReleasesBrowser
        key={activeTenant}
        rows={matrix.rows}
        envs={matrix.envs}
        resources={resources}
        onSelectResource={onSelectResource}
      />
    </div>
  );
}
