import { describe, expect, it } from 'vitest';
import {
  appliedByFromRaw,
  applierKindFromRaw,
  commitFromRevision,
  managedByFromRaw,
  ownerFromRaw,
  parseOciRevision,
  releaseCommitUrl,
  sourceLinkFromRaw,
  sourceRefFromRaw,
} from './sourceLink';

// A trimmed real OCIRepository raw (from the admin_test cluster).
const ociRaw = {
  spec: { url: 'oci://altinncr.azurecr.io/manifests/infra/altinn-uptime' },
  status: {
    artifact: {
      metadata: {
        'org.opencontainers.image.source': 'https://github.com/dis-way/gitops-manifests',
        'org.opencontainers.image.revision': 'main/51112f61c2a788a5d14a4c13a9a9437db219624e',
      },
    },
  },
};

describe('parseOciRevision', () => {
  it('parses the branch/sha form', () => {
    expect(parseOciRevision('main/51112f61c2a788a5d14a4c13a9a9437db219624e')).toEqual({
      ref: 'main',
      sha: '51112f61c2a788a5d14a4c13a9a9437db219624e',
    });
  });

  it('parses the branch@sha1:sha form', () => {
    expect(parseOciRevision('main@sha1:9f3c1a27d8')).toEqual({ ref: 'main', sha: '9f3c1a27d8' });
  });

  it('is empty for a missing revision', () => {
    expect(parseOciRevision(undefined)).toEqual({});
  });
});

describe('managedByFromRaw', () => {
  it('reads the Azure fluxConfiguration name from the clusterconfig labels', () => {
    // Trimmed real root Kustomization (from the admin_test cluster) — created
    // by an azapi fluxConfiguration, so Azure's extension stamped these labels.
    const raw = {
      metadata: {
        labels: {
          'clusterconfig.azure.com/name': 'dis-identity',
          'clusterconfig.azure.com/namespace': 'flux-system',
          'clusterconfig.azure.com/is-managed': 'true',
        },
      },
    };
    expect(managedByFromRaw(raw)).toEqual({ name: 'dis-identity', namespace: 'flux-system' });
  });

  it('is null without the labels', () => {
    expect(managedByFromRaw({ metadata: { labels: {} } })).toBeNull();
    expect(managedByFromRaw(undefined)).toBeNull();
  });
});

describe('sourceLinkFromRaw', () => {
  it('builds a repo + commit link from the OCI annotations', () => {
    const link = sourceLinkFromRaw(ociRaw);
    expect(link).not.toBeNull();
    expect(link?.repoUrl).toBe('https://github.com/dis-way/gitops-manifests');
    expect(link?.label).toBe('dis-way/gitops-manifests');
    expect(link?.shortSha).toBe('51112f6');
    expect(link?.commitUrl).toBe(
      'https://github.com/dis-way/gitops-manifests/commit/51112f61c2a788a5d14a4c13a9a9437db219624e',
    );
  });

  it('returns null when there are no source annotations', () => {
    expect(sourceLinkFromRaw({ status: { artifact: {} } })).toBeNull();
    expect(sourceLinkFromRaw(undefined)).toBeNull();
  });
});

describe('sourceRefFromRaw', () => {
  it('reads spec.sourceRef from a Kustomization', () => {
    const raw = { spec: { sourceRef: { kind: 'OCIRepository', name: 'altinn-uptime', namespace: 'flux-system' } } };
    expect(sourceRefFromRaw(raw)).toEqual({
      kind: 'OCIRepository',
      name: 'altinn-uptime',
      namespace: 'flux-system',
    });
  });

  it('returns null when there is no sourceRef', () => {
    expect(sourceRefFromRaw({ spec: {} })).toBeNull();
  });
});

describe('appliedByFromRaw', () => {
  it('reads the kustomize owner labels', () => {
    const raw = {
      metadata: {
        labels: {
          'kustomize.toolkit.fluxcd.io/name': 'grafana-operator-grafana-operator',
          'kustomize.toolkit.fluxcd.io/namespace': 'flux-system',
        },
      },
    };
    expect(appliedByFromRaw(raw)).toEqual({
      name: 'grafana-operator-grafana-operator',
      namespace: 'flux-system',
    });
  });

  it('returns null without the owner label', () => {
    expect(appliedByFromRaw({ metadata: { labels: {} } })).toBeNull();
    expect(appliedByFromRaw(undefined)).toBeNull();
  });
});

describe('ownerFromRaw', () => {
  it('picks the controlling owner reference', () => {
    const raw = {
      metadata: {
        ownerReferences: [
          { kind: 'ReplicaSet', name: 'x-abc', controller: false },
          { kind: 'OpenTelemetryCollector', name: 'otel', controller: true },
        ],
      },
    };
    expect(ownerFromRaw(raw)).toEqual({ kind: 'OpenTelemetryCollector', name: 'otel' });
  });

  it('falls back to the first reference, null when none', () => {
    expect(
      ownerFromRaw({ metadata: { ownerReferences: [{ kind: 'FluxConfig', name: 'cfg' }] } }),
    ).toEqual({ kind: 'FluxConfig', name: 'cfg' });
    expect(ownerFromRaw({ metadata: {} })).toBeNull();
  });
});

describe('applierKindFromRaw', () => {
  it('names a Kustomization from the kustomize owner labels', () => {
    expect(
      applierKindFromRaw({
        metadata: { labels: { 'kustomize.toolkit.fluxcd.io/name': 'frontend' } },
      }),
    ).toBe('Kustomization');
  });

  it('names a HelmRelease from the Helm managed-by label', () => {
    expect(
      applierKindFromRaw({ metadata: { labels: { 'app.kubernetes.io/managed-by': 'Helm' } } }),
    ).toBe('HelmRelease');
  });

  it('prefers kustomize when both markers are present, null when neither', () => {
    expect(
      applierKindFromRaw({
        metadata: {
          labels: {
            'kustomize.toolkit.fluxcd.io/name': 'x',
            'app.kubernetes.io/managed-by': 'Helm',
          },
        },
      }),
    ).toBe('Kustomization');
    expect(applierKindFromRaw({ metadata: { labels: {} } })).toBeNull();
  });
});

describe('commitFromRevision', () => {
  const repo = 'https://github.com/acme/gitops-manifests';

  it('links a git-sha revision to the commit', () => {
    expect(commitFromRevision(repo, 'main@sha1:9f3c1a2')).toBe(`${repo}/commit/9f3c1a2`);
    expect(commitFromRevision(`${repo}.git`, 'main/0a1b2c3d')).toBe(`${repo}/commit/0a1b2c3d`);
    expect(commitFromRevision(repo, '9f3c1a2')).toBe(`${repo}/commit/9f3c1a2`);
    // Git-sourced charts embed the sha as semver build metadata.
    expect(commitFromRevision(repo, '1.2.3+ab12cd34')).toBe(`${repo}/commit/ab12cd34`);
  });

  it('refuses OCI digests and chart versions', () => {
    expect(commitFromRevision(repo, 'at22@sha256:9f3c1a2e4b6d')).toBeNull();
    expect(commitFromRevision(repo, '1.4.7')).toBeNull();
  });

  it('needs a repo URL', () => {
    expect(commitFromRevision(undefined, 'main@sha1:9f3c1a2')).toBeNull();
    expect(commitFromRevision('oci://registry/repo', 'main@sha1:9f3c1a2')).toBeNull();
  });
});

describe('releaseCommitUrl', () => {
  const repo = 'https://github.com/acme/gitops-manifests';

  it('prefers the direct git-sha parse', () => {
    expect(releaseCommitUrl('main@sha1:9f3c1a2', repo)).toBe(`${repo}/commit/9f3c1a2`);
  });

  it('maps a currently-fetched OCI digest to the source origin commit', () => {
    const sources = [
      {
        revision: 'at22@sha256:9f3c1a2e4b6d',
        originSource: repo,
        originRevision: 'main/0a1b2c3d4e5f',
      },
    ];
    expect(releaseCommitUrl('at22@sha256:9f3c1a2e4b6d', repo, sources)).toBe(
      `${repo}/commit/0a1b2c3d4e5f`,
    );
    expect(releaseCommitUrl('at22@sha256:000aaa111bbb', repo, sources)).toBeNull();
  });
});
