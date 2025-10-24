# K6 Manifest Generation Action

This action generates the necessary k8s resources to run a k6 test in kubernetes.

Examples are available in the [altinn-platform-validation-tests](https://github.com/Altinn/altinn-platform-validation-tests/tree/main/.github/workflows) repository

## Inputs
## `config_file`

**Required** Path to where the configuration file is located.

## Optional Inputs
## `command_line_args`

**Optional** Command line arguments to pass to k6. Use this to override defaults or when running ad-hoc tests.

## Outputs
Generated manifests are written to a .dist/ folder at the root of the directory which you can then apply using [kubectl](https://kubernetes.io/docs/reference/kubectl/).

## Example usage

```
- name: Generate k8s manifests
  uses: Altinn/altinn-platform/actions/generate-k6-manifests@main
  with:
    config_file: "<path>/conf.yaml"
```
or
```
- name: Generate k8s manifests
  uses: Altinn/altinn-platform/actions/generate-k6-manifests@main
  with:
    config_file: "./services/k6/ad-hoc-test-conf.yaml"
    command_line_args: "--vus 23 --duration 7m"
```
