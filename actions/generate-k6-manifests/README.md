# K6 Manifest Generation Action

This action generates the necessary k8s resources to run a k6 test in kubernetes.
<!---
  TODO: Maintain the image with the needed tools in another workflow.
        This Dockerfile is too heavy.
-->
<!---
  TODO: Eventually, it might be worth it to write a simple CLI to add better validation.
        YOLO it while we handhold teams we onboard.
-->

## Inputs
## `test_script_filepath`

**Required** Path to where the main script file is located.

## `namespace`

**Required** The namespace assigned to you by the Platform team.

<!---
  TODO: How to handle config file.
  Per test, global, close to source, particular dir, etc..
-->

## Outputs
Generated manifests are written to a .dist/ folder at the root of the directory.

## Example usage

```
- name: Generate k8s manifests
  uses: ./actions/generate-k6-manifests/
  with:
    test_script_filepath: "./services/k6/first_test.js"
    namespace: platform
```

## Testing Manifest Generation locally
```
docker build -t k6/toolsimage \
                ./actions/generate-k6-manifests/ \
                && \
                docker run \
                -e INPUT_TEST_SCRIPT_FILEPATH="./services/k6/first_test.js" \
                -e INPUT_NAMESPACE="platform" \
                -v .:/github/workspace \
                --workdir /github/workspace \
                --rm \
                k6/toolsimage
```
## Development
The manifest generation is done with the help of Jsonnet.
Check Grafana Tanka's documentation for more context.
