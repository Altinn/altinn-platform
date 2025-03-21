# K6 Manifest Generation Action

This action generates the necessary k8s resources to run a k6 test in kubernetes.

## Inputs
## `config_file`

**Required** Path to where the test configurations is located.

## Optional Inputs
## `command_line_args`

**Required** Command line arguments to pass to k6. Use this to override defaults or when running ad-hoc tests.

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

Do not hardcode environment specific values in the test files (e.g. base url). The recommended approach is to pass a configuration option specificing the env to target (at21, yt01, tt02, prod, etc.)
and then use the injected env var.

e.g. if you want to target the yt01 environment, you can pass a configuration option in the conf.yaml file such that the correct env vars (such as the BASE_URL) will be available at run time.
```
# in your conf.yaml
deploy_env: yt01

# Certain environmental variables will be available for consumption such as the base url for the environment
BASE_URL: https://platform.yt01.altinn.cloud

# Consume it in your test
http.get(`${__ENV.BASE_URL}kuberneteswrapper/api/v1/Deployments`)
```
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
                -e INPUT_COMMAND_LINE_ARGS="foo=bar" \
                -e INPUT_CONFIG_FILE="services/k6/conf.yaml" \
                -v .:/github/workspace \
                --workdir /github/workspace \
                --rm \
                k6/toolsimage
```
or to debug inside the container
```
docker build -t k6/toolsimage \
                ./actions/generate-k6-manifests/ \
                && \
                docker run -it \
                -e INPUT_COMMAND_LINE_ARGS="foo=bar" \
                -e INPUT_CONFIG_FILE="services/k6/conf.yaml" \
                -v .:/github/workspace \
                --workdir /github/workspace \
                --rm \
                k6/toolsimage /bin/bash
```
## Development
The manifest generation is done with the help of Jsonnet.
Check Grafana Tanka's documentation for more context.

# Adoption
Start small
```
namespace: platform

test_definitions:
  - test_file: services/k6/test_k8s_wrapper_get_deployments.js
    contexts:
      - environment: yt01
```
This will generate the necessary ConfigMap and TestRun resources.

You can then continue in multiple ways. If you want to add more tests, just add another entry to the list.

```
namespace: platform

test_definitions:
  - test_file: services/k6/test_k8s_wrapper_get_deployments.js
    config_file: services/k6/test_configs/default.json
    contexts:
      - environment: at22
  - test_file: services/k6/test_k8s_wrapper_get_daemonsets.js
    config_file: services/k6/test_configs/default.json
    contexts:
      - environment: at22
```
This, generates a manifest similar to the above but with twice as many resources.

Or, you could add extra environments, for example at22
```
test_definitions:
  - test_file: services/k6/test_k8s_wrapper_get_deployments.js
    config_file: services/k6/test_configs/default.json
    contexts:
      - environment: at22
      - environment: yt01
```

A ConfigMap will be injected to the pod that will run the tests with the needed cluster specific variables. This is so you don't have to hardcode environmental specific values into the test code.

The contents of the ConfigMap looks similar to:
```
kubectl --context k6tests-cluster -n platform get cm deploy-environments-yt01 -o json | jq '.data'
{
  "BASE_URL": "https://platform.yt01.altinn.cloud"
}
kubectl --context k6tests-cluster -n platform get cm deploy-environments-at22 -o json | jq '.data'
{
  "BASE_URL": "https://platform.at22.altinn.cloud"
}
```

And therefore your js code can make use of these env vars like:

```
const res = http.get(`${__ENV.BASE_URL}/kuberneteswrapper/api/v1/Deployments`)
```

If there are more environmental specific values that make sense, you can always ask Platform to add them for you and for everyone else.

If you need to set environmental variables for a specific environment you can do it as such:

```
namespace: platform

test_definitions:
  - test_file: services/k6/test_k8s_wrapper_get_deployments.js
    config_file: services/k6/test_configs/default.json
    contexts:
      - environment: at22
        test_run:
          env:
          - name: FOO
            value: "BAR"
```

If you already have a workflow setup where you mostly just pass CLI args to k6s, you can do that by setting the Github Actions's input variable `command_line_args` to w.e string you were already using.

This is not, however, not the approach we recommend. In general, we recommend adding the configuration as config files and only using CLI args for example in quick tests or manually triggered tests. Check [K6 docs](https://grafana.com/docs/k6/latest/using-k6/k6-options/how-to/#order-of-precedence) to understand the order of precedence.

You can also override the defaults or add more options per environment like so:

```
test_definitions:
  - test_file: services/k6/test_k8s_wrapper_get_deployments.js
    config_file: services/k6/test_configs/default.json
    contexts:
      - environment: at22
        test_type:
          type: smoke
          enabled: true
          config_file: services/k6/test_configs/at_config.json
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

There are also some default scenarios you can re-use.
```
namespace: platform

test_definitions:
  - test_file: services/k6/test_k8s_wrapper_get_deployments.js
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
Reference conf file with all available options
```
namespace: <THE K8S NAMESPACE YOU HAVE PERMISSIONS ON>
test_definitions:
  - test_file: <RELATIVE PATH TO .JS FILE FROM REPO ROOT>
    config_file: <RELATIVE PATH TO .JSON FILE FROM REPO ROOT>
    contexts:
      - environment: <at22|at23|at24|yt01|prod|...>
        node_type: <spot|default>
        test_type:
          type: <smoke|soak|spike|breakpoint>
          enabled: <false|true>
        test_run:
          name: <NAME FOR THE TEST RUN>
          parallelism: 1
          resources:
            requests:
              memory: "1000Mi"
              cpu: "1"
          env:
            - name: FOO
              value: "BAR"
```
### SLO Generation

### Dry run

### Run action locally to generate manifests, apply manually

### I know what I'm doing - Tell me what are the integration portions and I will do the rest myself

### Running tests locally
`cd actions/generate-k6-manifests && /usr/local/go/bin/go test -v ./...`