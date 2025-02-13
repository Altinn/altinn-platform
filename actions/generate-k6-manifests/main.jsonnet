local k = import 'github.com/jsonnet-libs/k8s-libsonnet/1.32/main.libsonnet';
local yamlconf = std.parseYaml(std.extVar('userconfig'));
local k6ClusterYamlConf = std.parseYaml(std.extVar('k6clusterconfig'));
local suffix = std.extVar('suffix');
local extra_cli_args = std.extVar('extra_cli_args');
local testscriptdir = std.extVar('testscriptdir');
local unique_name = yamlconf.test_run.name + '-' + suffix;

local testrun = {
  new(): {
    apiVersion: 'k6.io/v1alpha1',
    kind: 'TestRun',
    metadata: {
      name: unique_name,
      namespace: yamlconf.namespace,
    },
    spec: {
      // cleanup: 'post',
      // arguments: std.format('--tag testid=%s --log-format json --out experimental-prometheus-rw',
      arguments: std.stripChars(
        std.format('--tag testid=%s --out experimental-prometheus-rw %s',
                   [unique_name, extra_cli_args]), ' '
      ),
      parallelism: yamlconf.test_run.parallelism,
      script: {
        configMap: {
          name: unique_name,
          file: 'archive.tar',
        },
      },
      runner: {
        env:
          [
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
          ] + [{ name: v.name, value: std.toString(v.value) } for v in yamlconf.test_run.env],  // TODO: Values from userconf should override the defaults. atm both get added
        metadata: {
          labels: {
            'k6-test': unique_name,
          },
        },
        resources: if std.objectHas(yamlconf, 'test_run') && std.objectHas(yamlconf.test_run, 'resources') then yamlconf.test_run.resources else {
          requests: {
            memory: '200Mi',
          },
        },
        envFrom: [{
          configMapRef: {
            name: 'deploy-environments-' + yamlconf.deploy_env,
          },
        }],
      },
    },
  },
  withNodeType(node_type): {
    spec+: {
      runner+: {
        nodeSelector: if std.objectHas(k6ClusterYamlConf.node_types, node_type)
        then { [v.label]: std.toString(v.value) for v in k6ClusterYamlConf.node_types[node_type].nodeSelector } else {},
        tolerations: if std.objectHas(k6ClusterYamlConf.node_types, node_type)
        then k6ClusterYamlConf.node_types[node_type].tolerations else [],
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
  'testrun.json': testrun.new()
                  + testrun.withNodeType(yamlconf.node_type)
                  + if std.objectHas(yamlconf, 'secret') then testrun.withEnvFromSecret(yamlconf.secret.name) else {},
}
