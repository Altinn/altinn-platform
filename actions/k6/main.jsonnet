local k = import 'github.com/jsonnet-libs/k8s-libsonnet/1.32/main.libsonnet';
local yamlconf = std.parseYaml(std.extVar('userconfig'));
local k6ClusterYamlConf = std.parseYaml(std.extVar('k6clusterconfig'));
local timestamp = std.extVar('timestamp');

local testrun = {
  new(name): {
    apiVersion: 'k6.io/v1alpha1',
    kind: 'TestRun',
    metadata: {
      name: yamlconf.test_run.name,
      namespace: yamlconf.namespace,
    },
    spec: {
      // TODO: VUs and Duration should be populated as ENV VAR
      arguments: std.format('--out experimental-prometheus-rw --vus=%s --duration=%s',
                            [yamlconf.test_run.vus, yamlconf.test_run.duration]),
      parallelism: yamlconf.test_run.parallelism,
      script: {
        configMap: {
          name: yamlconf.test_run.name,
          file: 'archive.tar',
        },
      },
      runner: {
        env:
          [
            {
              name: 'K6_PROMETHEUS_RW_SERVER_URL',
              value: k6ClusterYamlConf.prometheus_rw_server_url,
            },
            {
              name: 'K6_PROMETHEUS_RW_TREND_STATS',
              value: 'avg,min,med,max,count,p(95),p(99),p(99.5),p(99.9)',
            }
          ] + yamlconf.k6.env,
        metadata: {
          labels: {
            'k6-test': yamlconf.test_run.name,
          },
        },
      },
    },
  },
  withNodeType(node_type): {
    local has_tolerations = k6ClusterYamlConf["node_types"][node_type].has_tolerations,
    local has_labels = k6ClusterYamlConf["node_types"][node_type].has_labels,
    spec+: {
      runner+: {
        nodeSelector: if has_labels then k6ClusterYamlConf["node_types"][node_type].nodeSelector else {},
        tolerations: if has_tolerations then k6ClusterYamlConf["node_types"][node_type].tolerations else [],
      },
    },
  },
  withEnvFromSecret(secret_name): {
    spec+: {
      runner+: {
        envFrom+: [{
          secretRef: {
            name: secret_name,
          },
        }],
      },
    },
  },
};
{
  'testrun.json': testrun.new(yamlconf.test_run.name)
  + testrun.withNodeType(yamlconf.node_type)
  + if std.objectHas(yamlconf, "secret") then testrun.withEnvFromSecret(yamlconf.secret.name) else {}
}
