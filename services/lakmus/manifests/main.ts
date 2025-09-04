import * as cdk8s from 'cdk8s';
import { Construct } from 'constructs';
import * as kplus from 'cdk8s-plus-32';
import * as k8s from './imports/k8s';
import { App, YamlOutputType } from 'cdk8s';

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
            labels: { ...labels, 'azure.workload.identity/use': 'true'} 
          },
          spec: {
            serviceAccountName: sa.name,
            containers: [
              {
                name: 'lakmus',
                image: 'ghcr.io/altinn/altinn-platform/lakmus',
                args: ['--subscription-id=$(AZURE_SUBSCRIPTION_ID)'],
                env: [
                  { name: 'AZURE_SUBSCRIPTION_ID', value: '${AZURE_SUBSCRIPTION_ID}' },
                ],
                ports: [{ containerPort: 8080 }],
              },
            ],
          },
        },
      },
    });

  }
}

const app = new App({
  outputFileExtension: '.yaml',
  yamlOutputType: YamlOutputType.FILE_PER_CHART,
});
new LakmusChart(app, 'lakmus');
app.synth();
