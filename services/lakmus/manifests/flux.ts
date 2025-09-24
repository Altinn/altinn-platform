// manifests/flux.ts
import { App, Chart, ApiObject, YamlOutputType } from 'cdk8s';
import { Construct } from 'constructs';

import * as source from './imports/source.toolkit.fluxcd.io';
import * as kustomize from './imports/kustomize.toolkit.fluxcd.io';

class OciRepositoryChart extends Chart {
  constructor(scope: Construct, id: string) {
    super(scope, id);

    new source.OciRepository(this, 'OciRepository', {
      metadata: {
        name: 'lakmus',
        namespace: 'flux-system',
      },
      spec: {
        interval: '5m0s',
        provider: source.OciRepositorySpecProvider.AZURE,
        ref: { tag: 'v0.0.1' },
        timeout: '5m0s',
        url: 'oci://altinncr.azurecr.io/lakmus',
      },
    });
  }
}

class FluxKustomizeChart extends Chart {
  constructor(scope: Construct, id: string) {
    super(scope, id);

    new kustomize.Kustomization(this, 'Kustomization', {
      metadata: {
        name: 'lakmus',
        namespace: 'flux-system',
      },
      spec: {
        force: false,
        interval: '5m0s',
        path: './default',
        postBuild: {
          substitute: {
            LAKMUS_WORKLOAD_IDENTITY_CLIENT_ID: '${LAKMUS_WORKLOAD_IDENTITY_CLIENT_ID}',
            AZURE_SUBSCRIPTION_ID: '${AZURE_SUBSCRIPTION_ID}',
          },
        },
        prune: false,
        retryInterval: '5m0s',
        images: [
          {
            name: 'ghcr.io/altinn/altinn-platform/lakmus',
            newName: 'altinncr.azurecr.io/ghcr.io/altinn/altinn-platform/lakmus',
            newTag: 'v0.0.1',
          },
        ],
        sourceRef: {
          kind: kustomize.KustomizationSpecSourceRefKind.OCI_REPOSITORY,
          name: 'lakmus',
          namespace: 'flux-system',
        },
        targetNamespace: 'monitoring',
        timeout: '5m0s',
        wait: true,
      },
    });
  }
}

class RootKustomizationChart extends Chart {
  constructor(scope: Construct, id: string) {
    super(scope, id);

    new ApiObject(this, 'RootKustomization', {
      apiVersion: 'kustomize.config.k8s.io/v1beta1',
      kind: 'Kustomization',
        metadata: {
            name: 'lakmus',
            namespace: 'flux-system',
        },
      resources: [
        'oci-repository.yaml',
        'flux-kustomize.yaml',
      ],
    });
  }
}

const app = new App({
  outputFileExtension: '.yaml',
  yamlOutputType: YamlOutputType.FILE_PER_CHART,
});

new OciRepositoryChart(app, 'oci-repository');
new FluxKustomizeChart(app, 'flux-kustomize');
new RootKustomizationChart(app, 'kustomization');

app.synth();
