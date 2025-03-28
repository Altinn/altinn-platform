# K6 Manifest Generation Action

This action generates the necessary k8s resources to run a k6 test in kubernetes.

## Inputs
## `config_file`

**Required** Path to where the test configurations file is located.

## Optional Inputs
## `command_line_args`

**Optional** Command line arguments to pass to k6. Use this to override defaults or when running ad-hoc tests.

## Outputs
Generated manifests are written to a .dist/ folder at the root of the directory.

## Example usage

```
- name: Generate k8s manifests
  uses: ./actions/generate-k6-manifests/
  with:
    config_file: "./services/k6/conf.yaml"
```
or
```
- name: Generate k8s manifests
  uses: ./actions/generate-k6-manifests/
  with:
    config_file: "./services/k6/ad-hoc-test-conf.yaml"
    command_line_args: "--vus 23 --duration 7m"
```

## Recommended workflow
Keep test logic separate from test configuration. You should use a config file to host the [k6 configuration](https://grafana.com/docs/k6/latest/using-k6/k6-options/reference/#config).
To override configuration options, you have 3 options. You can pass an extra config file that will override whatever options were configured on the default configuration file or add new ones that didn't exist before. You can pass in env variables in the conf.yaml. And lastly, you can pass command line arguments.

Do not hardcode environment specific values in the test files (e.g. base url). The recommended approach is to pass a configuration option specificing the env to target (at22, yt01, tt02, prod, etc.)
and then use the injected env var.

e.g. if you want to target the yt01 environment, you can pass a configuration option in the conf.yaml file such that the correct env vars (such as the BASE_URL) will be available at run time.
```
# in your conf.yaml
environment: yt01

# Certain environmental variables will be available for consumption such as the base url for the environment
BASE_URL: https://platform.yt01.altinn.cloud

# Consume it in your test
http.get(`${__ENV.BASE_URL}kuberneteswrapper/api/v1/Deployments`)
```

Check the [example configuration files](altinn-platform/actions/generate-k6-manifests/cmd/example_configfiles) and what they [generate](altinn-platform/actions/generate-k6-manifests/cmd/expected_generated_files) for examples of what's possible.
## Default configurations
Explain how to overide default configs, so tests can be re-used across envs and test types.

Explain how to re-use traffic patterns

Explain PR remote write, report metrics, resource, etc.

## Testing Manifest Generation locally
`docker build -t k6/toolsimage ./actions/generate-k6-manifests/ && docker run -u $(id -u ${USER}):$(id -g ${USER}) -v .:/github/workspace --workdir /github/workspace --rm -e INPUT_CONFIG_FILE="services/k6/conf.yaml" k6/toolsimage`
```
docker build -t k6/toolsimage \
                ./actions/generate-k6-manifests/ \
                && \
                docker run \
                -u $(id -u ${USER}):$(id -g ${USER}) \
                -v .:/github/workspace \
                --workdir /github/workspace \
                --rm \
                -e INPUT_CONFIG_FILE="services/k6/conf.yaml" \
                k6/toolsimage
```
or to debug inside the container
```
docker build -t k6/toolsimage \
                ./actions/generate-k6-manifests/ \
                && \
                docker run -it \
                -u $(id -u ${USER}):$(id -g ${USER}) \
                -v .:/github/workspace \
                --workdir /github/workspace \
                --rm \
                -e INPUT_CONFIG_FILE="services/k6/conf.yaml" \
                k6/toolsimage /bin/bash
```
## Development
The manifest generation is done with the help of [Jsonnet](https://jsonnet.org/).
Check Grafana [Tanka](https://grafana.com/oss/tanka/)'s documentation for more context.

# Adoption
Minimum configuration file:
```
namespace: Platform will assign a namespace for you.
test_file: The path to the test file you want to run.
environment: the environment the test will run against (at22, yt01, etc.).
```
```
namespace: platform

test_definitions:
  - test_file: services/k6/k8s_wrapper/get_deployments.js
    contexts:
      - environment: yt01
```
This will generate the necessary k8s resources (you can generate them locally as well and see exactly what gets generated in the .dist/ and .conf/ dirs at the root of the repository).

You can then continue in multiple ways. If you want to add more tests, just add another entry to the list.

```
namespace: platform

test_definitions:
  - test_file: services/k6/k8s_wrapper/get_deployments.js
    contexts:
      - environment: yt01
  - test_file: services/k6/k8s_wrapper/get_daemonsets.js
    contexts:
      - environment: yt01
```
This, generates a manifest similar to the above but with twice as many resources.

Or, you could add extra environments, for example at22
```
test_definitions:
  - test_file: services/k6/k8s_wrapper/get_deployments.js
    contexts:
      - environment: yt01
      - environment: at22
```

A ConfigMap will be injected to the pod that will run the tests with the needed cluster specific variables. This is so you don't have to hardcode environmental specific values into the test code. If you need a new variable open an issue so we can add it for you and others that might need it in the future.

The contents of the ConfigMap looks similar to:
```
kubectl --context k6tests-cluster -n platform get configmap deploy-environments-yt01 -o json | jq '.data.BASE_URL'
"https://platform.yt01.altinn.cloud"

kubectl --context k6tests-cluster -n platform get configmap deploy-environments-at22 -o json | jq '.data.BASE_URL'
"https://platform.at22.altinn.cloud"
```

And therefore your test code can make use of these env vars like so:

```
const res = http.get(`${__ENV.BASE_URL}/kuberneteswrapper/api/v1/Deployments`)
```

If you need to set environmental variables for a specific environment you can do it like so:

```
namespace: platform

test_definitions:
  - test_file: services/k6/k8s_wrapper/get_deployments.js
    contexts:
      - environment: at22
        test_run:
          env:
          - name: FOO
            value: "BAR"
```
Which you can then use in your test code
```
const bar = __ENV.FOO
```

If you already have a workflow setup where you mostly just pass CLI args to K6, you can do that by setting the Github Actions's input variable `command_line_args` to w.e string you were already using before.

This is not, however, the approach we recommend. In general, we recommend adding the configuration as config files and only using CLI args for example in quick tests or manually triggered tests. Check [K6 docs](https://grafana.com/docs/k6/latest/using-k6/k6-options/how-to/#order-of-precedence) to understand the order of precedence.

You can also override the defaults or add more options per environment like so:
```
test_definitions:
  - test_file: services/k6/k8s_wrapper/get_deployments.js
    config_file: services/k6/test_configs/default.json #         # A config_file with options for all tests
    contexts:
      - environment: at22
        test_type:
          type: custom
          enabled: true
          config_file: services/k6/test_configs/at_config.json   # Environment specific options
```
Given the original default.json as
```
{
    "stages": [
        {
            "duration": "10m",
            "target": 100
        }
    ]
}
```
and at_config.json as
```
{
    "thresholds": {
        "http_req_duration": [
            "p(95)<1000"
        ]
    }
}
```
the resulting config would be:
```
{
  "stages": [
    {
      "duration": "10m",
      "target": 100
    }
  ],
  "thresholds": {
    "http_req_duration": [
      "p(95)<1000"
    ]
  }
}
```
If at_config.json had a "stages" block config, that would override the "stages" inherited from default.json.

There are also some default scenarios you can re-use (**This functionality is still in consideration; this may or may not be supported in the future**).
```
namespace: platform

test_definitions:
  - test_file: services/k6/k8s_wrapper/get_deployments.js
    contexts:
      - environment: at22
        test_type:
          type: breakpoint
          enabled: true
```
Will automatically add a config file like so:
```
{
  "executor": "ramping-arrival-rate",
  "stages": [
    {
      "duration": "2h",
      "target": 20000
    }
  ],
  "thresholds": {
    "http_req_duration": [
      {
        "abortOnFail": true,
        "delayAbortEval": "1s",
        "threshold": "p(95)<200"
      }
    ],
    "http_req_failed": [
      {
        "abortOnFail": true,
        "delayAbortEval": "1s",
        "threshold": "rate<0.01"
      }
    ]
  }
}
```

### SLO Generation
**This feature is still in development**

It's also possible to generate automatic SLOs based on the K6 metrics.

Since this is an abstraction over Pyrra, a lot of the boilerplate will be abstracted away in a way that only the bare minimum configuration is required.

#### HTTP responses
3 parameters would likely make sense to configure: the [metric filter](https://prometheus.io/docs/prometheus/latest/querying/basics/), the SLO target, and the time window to consider.
```
indicator:
  ratio:
    errors:
      metric: k6_http_reqs_total{ name=~".*/kuberneteswrapper/api/v1/Deployments", status=~"5...|418" }
    total:
      metric: k6_http_reqs_total{ name=~".*/kuberneteswrapper/api/v1/Deployments" }
target: "99.0"
window: 7d
```
#### Latency
`TODO`

### Dry run

### Run action locally to generate manifests, apply manually

### I know what I'm doing - Tell me what the integration points are and I will do the rest myself
- You can see which permissions you have within the cluster [here](https://github.com/Altinn/altinn-platform/blob/f94b376bb3222a98e9100ca258a9670b6ed32237/infrastructure/adminservices-test/altinn-monitor-test-rg/k6_tests_rg_rbac.tf#L1-L31).
- You can see how the k6-operator works [here](https://grafana.com/docs/k6/latest/testing-guides/running-distributed-tests/).
- Suggestions:
  - Cleanup after yourself, e.g. set the `cleanup: post` [option](https://grafana.com/docs/k6/latest/testing-guides/running-distributed-tests/#5-run-your-test) to the TestRun CR.
  - Tag your ConfigMaps with a [unique label](https://kubernetes.io/docs/reference/kubectl/generated/kubectl_get/#options) such that it's easy to delete the ConfigMaps. e.g.:
    - kubectl --context k6tests-cluster -n platform get cm -l k6-test-configmap=true
    - kubectl --context k6tests-cluster -n platform delete cm -l k6-test-configmap=true
- You can send metrics to [Prometheus](https://grafana.com/docs/k6/latest/results-output/real-time/prometheus-remote-write/#send-test-metrics-to-a-remote-write-endpoint) via prometheus-remote-write.
  - K6_PROMETHEUS_RW_SERVER_URL: http://kube-prometheus-stack-prometheus.monitoring:9090/api/v1/write


### Running tests locally
`cd actions/generate-k6-manifests && /usr/local/go/bin/go test -v ./...`
