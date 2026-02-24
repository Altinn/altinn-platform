import * as cdk8s from 'cdk8s';
import { Construct } from 'constructs';
import * as kplus from 'cdk8s-plus-32';
import * as k8s from './imports/k8s';
import { ApiObject, App, YamlOutputType } from 'cdk8s';

// Just to mimic kubebuilder's way
const CONTAINER_IMAGE = 'controller:latest';

export class LakmusChart extends cdk8s.Chart {
  constructor(scope: Construct, id: string) {
    super(scope, id);

    const namespace = 'monitoring';
    const name = 'lakmus';
    const labels = {
        app: name,
        owner: 'platform', 
    };

    const baseMetadata: cdk8s.ApiObjectMetadata = {
      name,
      namespace,
      labels,
    };

    const sa = new kplus.ServiceAccount(this, 'sa', {
      metadata: {
        ...baseMetadata,
        annotations: {
          ...(baseMetadata.annotations ?? {}),
          'azure.workload.identity/client-id': '${LAKMUS_WORKLOAD_IDENTITY_CLIENT_ID}',
        },
      },
    });

    new k8s.KubeDeployment(this, 'deployment', {
      metadata: baseMetadata,
      spec: {
        replicas: 1,
        selector: { matchLabels: labels },
        template: {
          metadata: { 
            labels: { ...labels, 'azure.workload.identity/use': 'true'},
            annotations: {
              'cluster-autoscaler.kubernetes.io/safe-to-evict': 'true',
            },
          },
          spec: {
             serviceAccountName: sa.name,
             automountServiceAccountToken: false,
             enableServiceLinks: false,
             securityContext: {
               runAsNonRoot: true,
               seccompProfile: { type: 'RuntimeDefault' },
             },
             containers: [
               {
                 name: 'lakmus',
                 image: CONTAINER_IMAGE,
                 args: ['--subscription-id=$(AZURE_SUBSCRIPTION_ID)'],
                 env: [
                   { name: 'AZURE_SUBSCRIPTION_ID', value: '${AZURE_SUBSCRIPTION_ID}' },
                 ],
                 securityContext: {
                   allowPrivilegeEscalation: false,
                   readOnlyRootFilesystem: true,
                   capabilities: { drop: ['ALL'] },
                 },
                 ports: [{ name: 'http', containerPort: 8080 }],
               },
             ],
           },
        },
      },
    });

    // TODO: find the crd for this and import it properly
    new ApiObject(this, 'lakmus-podmonitor', {
      apiVersion: 'azmonitoring.coreos.com/v1',
      kind: 'PodMonitor',
      metadata: {
        name,
        namespace,
        labels,
      },
      spec: {
        selector: {
          matchLabels: labels,
        },
        namespaceSelector: { any: true },
        podMetricsEndpoints: [
          {
            port: 'http',
            path: '/metrics',
            interval: '30s',
          },
        ],
      },
    });

  }
}

class BaseKustomizationChart extends cdk8s.Chart {
  constructor(scope: Construct, id: string) {
    super(scope, id);

    new ApiObject(this, 'Kustomization', {
      apiVersion: 'kustomize.config.k8s.io/v1beta1',
      kind: 'Kustomization',
      metadata: {
        name: 'lakmus',
      },
      resources: ['lakmus.yaml'],
    });
  }
}

const app = new App({
  outputFileExtension: '.yaml',
  yamlOutputType: YamlOutputType.FILE_PER_CHART,
});
new LakmusChart(app, 'lakmus');
new BaseKustomizationChart(app, 'kustomization');
app.synth();
