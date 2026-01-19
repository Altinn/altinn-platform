local k = import 'github.com/jsonnet-libs/k8s-libsonnet/1.32/main.libsonnet';
local k6ClusterYamlConf = std.parseYaml(std.extVar('k6clusterconfig'));
// Global
local unique_name = std.extVar('unique_name');
local configmap_name = std.extVar('configmap_name');
local test_name = std.extVar('test_name');
local test_scope = std.extVar('test_scope');
local testfilename = std.extVar('testfilename');
local manifest_generation_timestamp = std.extVar('manifest_generation_timestamp');
local namespace = std.extVar('namespace');
local deploy_env = std.extVar('deploy_env');
// Testrun
local testid = if std.length(std.extVar('testid')) > 0 then std.extVar('testid') else unique_name;  // Only used in metrics
local parallelism = std.parseInt(std.extVar('parallelism'));
local extra_env_vars = std.parseYaml(std.extVar('extra_env_vars'));
local secret_references = std.parseYaml(std.extVar('secret_references'));
local resources = std.parseYaml(std.extVar('resources'));
local node_type = std.extVar('node_type');
//
local extra_cli_args = std.extVar('extra_cli_args');
local default_env = [
  {
    name: 'K6_NO_USAGE_REPORT',
    value: 'true',
  },
  {
    name: 'K6_PROMETHEUS_RW_SERVER_URL',
    value: k6ClusterYamlConf.prometheus_rw_server_url,
  },
  {
    name: 'K6_PROMETHEUS_RW_TREND_STATS',
    value: 'avg,min,med,max,count,p(95),p(99),p(99.5),p(99.9)',
  },
  {
    name: 'MANIFEST_GENERATION_TIMESTAMP',
    value: manifest_generation_timestamp,
  },
  {
    name: 'NAMESPACE',
    value: namespace,
  },
  {
    name: 'TESTID',
    value: testid,
  },
  {
    name: 'ENVIRONMENT',
    value: deploy_env,
  },
  {
    name: 'RUNNING_IN_K8S',
    value: 'true',
  },
  {
    name: 'TESTFILENAME',
    value: testfilename,
  },
  {
    name: 'TEST_SCOPE',
    value: test_scope,
  },
];

local common_labels = {
  uniq_name: unique_name,
  'generated-by': 'k6-action-image',
};
local common_annotations = {
  'k6-action-image/test_name': test_name,
  'k6-action-image/test_scope': test_scope,
  'k6-action-image/testid': testid,
  'k6-action-image/testfilename': testfilename,
};

local slo = {
  new(slo_name, team, application, url): {
    apiVersion: 'pyrra.dev/v1alpha1',
    kind: 'ServiceLevelObjective',
    metadata: {
      name: slo_name,
      namespace: namespace,
      labels: {
        prometheus: 'k8s',
        role: 'alert-rules',
        'pyrra.dev/team': team,
        'pyrra.dev/application': application,
        release: 'kube-prometheus-stack',  // Important, otherwise the Prometheus instance won't pick it up
        'generated-by': 'k6-action-image',
      },
    },
    spec: {
      target: '99.0',
      window: '7d',
      indicator: {
        ratio: {
          errors: {
            // metric: 'k6_http_reqs_total{ name=~".*/kuberneteswrapper/api/v1/Deployments", status=~"5...|418" }',
            metric: std.format('k6_http_reqs_total{ name=~%s, status=~"5...|418" }', url),
          },
          total: {
            // metric: 'k6_http_reqs_total{ name=~".*/kuberneteswrapper/api/v1/Deployments" }',
            metric: std.format('k6_http_reqs_total{ name=~%s }', url),
          },
        },
      },
    },
  },
};

local testrun = {
  new(): {
    apiVersion: 'k6.io/v1alpha1',
    kind: 'TestRun',
    metadata: {
      name: unique_name,
      namespace: namespace,
      labels: common_labels,
      annotations: common_annotations,
    },

    spec: {
      cleanup: 'post',
      arguments: std.stripChars(
        std.format('--tag testid=%s --tag namespace=%s --tag deploy_env=%s --tag test_name=%s --tag test_scope=%s --out experimental-prometheus-rw %s',
                   [testid, namespace, deploy_env, test_name, test_scope, extra_cli_args]), ' '
      ),
      parallelism: parallelism,
      script: {
        configMap: {
          name: configmap_name,
          file: 'archive.tar',
        },
      },
      runner: {
        env: default_env,
        metadata: {
          labels: common_labels,
          annotations: common_annotations,
        },
        resources: resources,
        envFrom+: [{
          configMapRef: {
            name: 'deploy-environments-' + deploy_env,
          },
        }],
      },
    },
  },
  withNodeType(node_type): {
    spec+: {
      runner+: {
        nodeSelector: { [v.label]: std.toString(v.value) for v in k6ClusterYamlConf.node_types[node_type].nodeSelector },
        tolerations: k6ClusterYamlConf.node_types[node_type].tolerations,
      },
    },
  },
  withEnvFromSecret(secret_references): {
    spec+: {
      runner+: {
        envFrom+: [
          {
            secretRef: {
              name: secret_name,
            },
          }
          for secret_name in secret_references
        ],
      },
    },
  },
  withExtraEnv(): {
    local extraEnvVarsSet = std.set([{ name: v.name, value: std.toString(v.value) } for v in extra_env_vars], keyF=function(x) x.name),
    local defaultEnvSet = std.set(default_env, keyF=function(x) x.name),
    local newEnv =
      std.setUnion(
        extraEnvVarsSet, defaultEnvSet, keyF=function(x) x.name
      ),
    spec+: {
      runner+: {
        env: newEnv,
      },
    },
  },
  withCustomImage(customImage): {
    spec+: {
      runner+: {
        image: customImage,
      },
    },
  },
};
{
  'testrun.json': testrun.new()
                  + testrun.withNodeType(node_type)
                  + testrun.withExtraEnv()
                  + (if std.length(secret_references) != 0 then testrun.withEnvFromSecret(secret_references) else {})
                  + (if std.length(std.extVar('image_name')) > 0 then testrun.withCustomImage(std.extVar('image_name')) else {}),


  // TODO: Disable for now since most of the things are hardcoded
  //'slo.json': if false then slo.new('k8-wrapper-deployments-query', 'platform', 'k8s-wrapper', '.*/kuberneteswrapper/api/v1/Deployments') else null,
}
