import { useMemo, useState } from 'react';
import { Alert, Heading, Paragraph, Tabs } from '@digdir/designsystemet-react';
import { Menu } from 'lucide-react';
import { useMock } from './api';
import type { Artifact, Resource } from './api/types';
import logoColor from './assets/dis-logo-color.png';
import logoWhite from './assets/dis-logo-white.png';
import { useArtifacts } from './hooks/useArtifacts';
import { useFleet } from './hooks/useFleet';
import { useHashRoute } from './hooks/useHashRoute';
import type { ResourceRef } from './hooks/useResourceDetail';
import { DIS_PRODUCTS } from './lib/flux';
import { disEnvsOf, disTenantsOf } from './lib/disResources';
import { ArtifactDialog } from './components/ArtifactDialog';
import { DeploymentMatrix } from './components/DeploymentMatrix';
import { DetailDialog } from './components/DetailDialog';
import { DisResourcesView } from './components/DisResourcesView';
import { DisScope } from './components/DisScope';
import { HomeView } from './components/HomeView';
import { LeftNav, type Section } from './components/LeftNav';
import { MatrixSkeleton } from './components/MatrixSkeleton';
import { MockBanner } from './components/MockBanner';
import { ReleasesOverview } from './components/ReleasesBrowser';
import { StaleBanner } from './components/StaleBanner';
import { SyncrootsView } from './components/SyncrootsView';

export function App() {
  const { clusters, resources, loading, error } = useFleet();
  const [selected, setSelected] = useState<ResourceRef | null>(null);
  const [artifact, setArtifact] = useState<Artifact | null>(null);
  const [route] = useHashRoute();
  const [navCollapsed, setNavCollapsed] = useState(false);

  // Pages under the Syncroots section keep its nav item active.
  const section: Section =
    route.view === 'syncroot' || route.view === 'kustomization' ? 'syncroots' : route.view;
  const [disTenant, setDisTenant] = useState('');
  const [disEnv, setDisEnv] = useState('');

  // The DIS scope (tenant + environment) is shared across Home and every
  // per-product view, so it persists as you move between products.
  const disTenants = useMemo(() => disTenantsOf(resources), [resources]);
  const activeTenant = disTenants.includes(disTenant) ? disTenant : (disTenants[0] ?? '');
  const disEnvs = useMemo(
    () => disEnvsOf(resources, activeTenant || undefined),
    [resources, activeTenant],
  );
  const activeEnv = disEnvs.includes(disEnv) ? disEnv : (disEnvs[0] ?? '');
  const disCluster = activeTenant && activeEnv ? `${activeTenant}_${activeEnv}` : '';

  return (
    <div className="app">
      <header className="appbar">
        <button
          type="button"
          className="appbar__toggle"
          aria-label={navCollapsed ? 'Expand menu' : 'Collapse menu'}
          aria-expanded={!navCollapsed}
          onClick={() => setNavCollapsed((c) => !c)}
        >
          <Menu size="1.5rem" aria-hidden />
        </button>
        <Heading level={1} data-size="xs" className="appbar__brand">
          <picture>
            <source media="(prefers-color-scheme: dark)" srcSet={logoWhite} />
            <img src={logoColor} alt="DIS" className="appbar__logo" />
          </picture>
          <span className="appbar__brand-text">Console</span>
        </Heading>
      </header>

      <div className="app__body">
        <LeftNav active={section} collapsed={navCollapsed} />
        <div className="app__content">
          <div className="app__content-inner">
            {useMock && <MockBanner />}
            {!loading && <StaleBanner clusters={clusters} />}
            {error && (
              <Alert data-color="danger">
                <Paragraph>Failed to load fleet data: {error}</Paragraph>
              </Alert>
            )}

            {loading ? (
              <MatrixSkeleton />
            ) : section === 'syncroots' ? (
              <Syncroots
                resources={resources}
                onSelectResource={setSelected}
                onSelectArtifact={setArtifact}
              />
            ) : section === 'deployments' ? (
              <Tabs defaultValue="releases">
                <Tabs.List>
                  <Tabs.Tab value="releases">Releases</Tabs.Tab>
                  <Tabs.Tab value="matrix">Matrix</Tabs.Tab>
                </Tabs.List>
                <Tabs.Panel value="releases">
                  <ReleasesOverview resources={resources} onSelectResource={setSelected} />
                </Tabs.Panel>
                <Tabs.Panel value="matrix">
                  <DeploymentMatrix resources={resources} onSelectCell={setSelected} />
                </Tabs.Panel>
              </Tabs>
            ) : section === 'home' ? (
              <HomeView clusters={clusters} resources={resources} />
            ) : (
              <>
                <DisScope
                  tenants={disTenants}
                  activeTenant={activeTenant}
                  onTenant={setDisTenant}
                  envs={disEnvs}
                  activeEnv={activeEnv}
                  onEnv={setDisEnv}
                />
                {section === 'databases' && (
                  <DisResourcesView
                    resources={resources}
                    cluster={disCluster}
                    kinds={DIS_PRODUCTS.databases}
                    title="Databases"
                  />
                )}
                {section === 'identities' && (
                  <DisResourcesView
                    resources={resources}
                    cluster={disCluster}
                    kinds={DIS_PRODUCTS.identities}
                    title="Identities"
                  />
                )}
                {section === 'apim' && (
                  <DisResourcesView
                    resources={resources}
                    cluster={disCluster}
                    kinds={DIS_PRODUCTS.apim}
                    title="APIM"
                  />
                )}
                {section === 'vaults' && (
                  <DisResourcesView
                    resources={resources}
                    cluster={disCluster}
                    kinds={DIS_PRODUCTS.vaults}
                    title="Vaults"
                  />
                )}
              </>
            )}
          </div>
        </div>
      </div>

      <DetailDialog
        selected={selected}
        resources={resources}
        onSelect={setSelected}
        onClose={() => setSelected(null)}
      />
      <ArtifactDialog selected={artifact} onClose={() => setArtifact(null)} />
    </div>
  );
}

/** Lazily loads the base-layer artifacts when the Syncroots section opens. */
function Syncroots({
  resources,
  onSelectResource,
  onSelectArtifact,
}: {
  resources: Resource[];
  onSelectResource: (ref: ResourceRef) => void;
  onSelectArtifact: (artifact: Artifact) => void;
}) {
  const { artifacts, loading, error } = useArtifacts();
  if (loading) return <MatrixSkeleton />;
  if (error) {
    return (
      <Alert data-color="danger">
        <Paragraph>Failed to load artifacts: {error}</Paragraph>
      </Alert>
    );
  }
  return (
    <SyncrootsView
      artifacts={artifacts}
      resources={resources}
      onSelectResource={onSelectResource}
      onSelectArtifact={onSelectArtifact}
    />
  );
}
