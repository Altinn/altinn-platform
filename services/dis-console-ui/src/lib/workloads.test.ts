import { describe, expect, it } from 'vitest';
import type { Resource } from '../api/types';
import { imageLabel, parseImageRef, syncrootWorkloads, workloadsOf } from './workloads';

const app = { kind: 'Kustomization', namespace: 'acme', name: 'frontend', tenant: 'acme' };

const dep = (
  cluster: string,
  images: { container: string; image: string }[],
  ready: 'True' | 'False' | 'Unknown' = 'True',
): Resource => ({
  kind: 'Deployment',
  apiVersion: 'apps/v1',
  namespace: 'acme',
  name: 'frontend',
  ready,
  suspended: false,
  cluster,
  appliedBy: { name: 'frontend', namespace: 'acme' },
  images,
});

describe('parseImageRef / imageLabel', () => {
  it('splits repo, tag and digest', () => {
    expect(parseImageRef('ghcr.io/acme/frontend:1.42.0')).toEqual({
      repo: 'ghcr.io/acme/frontend',
      tag: '1.42.0',
      digest: undefined,
    });
    expect(parseImageRef('ghcr.io/acme/frontend:1.42.0@sha256:abc123')).toEqual({
      repo: 'ghcr.io/acme/frontend',
      tag: '1.42.0',
      digest: 'sha256:abc123',
    });
  });

  it('does not mistake a registry port for a tag', () => {
    expect(parseImageRef('registry.local:5000/acme/frontend')).toEqual({
      repo: 'registry.local:5000/acme/frontend',
      digest: undefined,
    });
  });

  it('labels by tag, short digest, or the implicit latest', () => {
    expect(imageLabel('ghcr.io/a/b:1.2.3')).toBe('1.2.3');
    expect(imageLabel('ghcr.io/a/b@sha256:0123456789abcdef')).toBe('0123456789ab');
    expect(imageLabel('ghcr.io/a/b')).toBe('latest');
  });
});

describe('workloadsOf', () => {
  it('groups an app workload across environments and flags image drift', () => {
    const rows = workloadsOf(
      [
        dep('acme_at22', [
          { container: 'frontend', image: 'ghcr.io/acme/frontend:1.42.0' },
          { container: 'otel', image: 'ghcr.io/acme/otel:0.8.1' },
        ]),
        dep('acme_production', [
          { container: 'frontend', image: 'ghcr.io/acme/frontend:1.41.2' },
          { container: 'otel', image: 'ghcr.io/acme/otel:0.8.1' },
        ]),
      ],
      app,
      ['at22', 'production'],
    );
    expect(rows).toHaveLength(1);
    expect(rows[0].containers).toEqual(['frontend', 'otel']);
    expect(rows[0].driftContainers.has('frontend')).toBe(true);
    expect(rows[0].driftContainers.has('otel')).toBe(false);
    expect(rows[0].cells.at22?.images.frontend).toBe('ghcr.io/acme/frontend:1.42.0');
  });

  it('matches only workloads applied by the app, in the app tenant', () => {
    const other = { ...dep('acme_at22', []), appliedBy: { name: 'other', namespace: 'acme' } };
    const wrongTenant = dep('fabrikam_at22', [{ container: 'c', image: 'x:1' }]);
    const rows = workloadsOf([other, wrongTenant], app, ['at22']);
    expect(rows).toHaveLength(0);
  });

  it('matches HelmRelease apps too once workloads carry Helm ownership', () => {
    const chartDep: Resource = {
      ...dep('fabrikam_at22', [{ container: 'payments', image: 'ghcr.io/fabrikam/payments:0.9.3' }]),
      namespace: 'fabrikam-platform',
      name: 'payments',
      appliedBy: { name: 'payments', namespace: 'fabrikam-platform' },
    };
    const rows = workloadsOf(
      [chartDep],
      { namespace: 'fabrikam-platform', name: 'payments', tenant: 'fabrikam' },
      ['at22'],
    );
    expect(rows).toHaveLength(1);
    expect(rows[0].cells.at22?.images.payments).toBe('ghcr.io/fabrikam/payments:0.9.3');
  });

  it('includes chart workloads of HelmReleases folded into the app row (extraOwners)', () => {
    const chartDep: Resource = {
      ...dep('acme_at22', [{ container: 'gw', image: 'ghcr.io/acme/gw:1.4.7' }]),
      name: 'gateway',
      namespace: 'acme-edge',
      appliedBy: { name: 'api-gateway-chart', namespace: 'acme-edge' },
    };
    const withoutOwner = workloadsOf(
      [chartDep],
      { namespace: 'acme', name: 'api-gateway', tenant: 'acme' },
      ['at22'],
    );
    expect(withoutOwner).toHaveLength(0);
    const rows = workloadsOf(
      [chartDep],
      { namespace: 'acme', name: 'api-gateway', tenant: 'acme' },
      ['at22'],
      [{ name: 'api-gateway-chart', namespace: 'acme-edge' }],
    );
    expect(rows).toHaveLength(1);
    expect(rows[0].cells.at22?.images.gw).toBe('ghcr.io/acme/gw:1.4.7');
  });
});

describe('syncrootWorkloads', () => {
  it('includes workloads applied directly by the root Kustomization (no app of their own)', () => {
    const rootDep: Resource = {
      ...dep('acme_at22', [{ container: 'agent', image: 'acr.io/dis/agent:1.6.0' }]),
      namespace: 'product-dis',
      name: 'dis-console-agent',
      appliedBy: { name: 'acme-apps', namespace: 'flux-system' },
    };
    const row = {
      cells: {
        at22: {
          artifact: {
            cluster: 'acme_at22',
            namespace: 'flux-system',
            name: 'acme-syncroot',
            url: 'oci://reg/acme/syncroot',
            class: 'product-syncroot',
            ready: 'True' as const,
            suspended: false,
            kustomizations: [
              { name: 'acme-apps', namespace: 'flux-system', ready: 'True' as const, suspended: false },
            ],
          },
          inFlight: false,
        },
      },
    };
    const rows = syncrootWorkloads([rootDep], row, ['at22']);
    expect(rows).toHaveLength(1);
    expect(rows[0].name).toBe('dis-console-agent');
    expect(rows[0].app).toBe('acme-apps');
    expect(rows[0].cells.at22?.images.agent).toBe('acr.io/dis/agent:1.6.0');
  });
});
