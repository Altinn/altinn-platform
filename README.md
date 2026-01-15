# Altinn Platform

## Altinn Products
To configure or add identity federation (GitHub to Azure), Azure IAM, and handle Terraform state for a product in Altinn, modify the `products.yaml` file and create a pull request.

## Release Please

We have setup Release Please in our repo to automate and simplify the releasing of different artifacts.

Since we have a monorepo we have opted to use the Manifest Driven release-please setup.

This setup consists of two files:
* [.release-please-manifest.json](./.release-please-manifest.json) that contains the latest release version of all packages
* [release-please-config.json](./release-please-config.json) that contains the configuration of all the packages handled by release-please

Further documentation about this can be found in the documentation for [Manifest Driven release-please](https://github.com/googleapis/release-please/blob/main/docs/manifest-releaser.md)

### Post-Release Workflow Configuration

We have created our own logic to simplify the dispatching of pipelines after a release is created. This is a fairly simple github script and json file that we have implemented

Control downstream actions after a release using `release-please-post-config.json`.

#### Configuration
Map release paths (using Regex) to workflows.

**Note:** All matching patterns are executed. If a path matches multiple keys, all associated pipelines will be triggered.

##### Configuration Example

```json
{
  "post-release-hooks": {
    // Example 1: Simple exact match, no inputs required
    "pkg/docs": {
      "dispatch-pipelines": [
        {
          "filename": "deploy-docs.yaml"
        }
      ]
    },
    // Example 2: Regex match (all packages under pkg/) with inputs
    "pkg/.*": {
      "dispatch-pipelines": [
        {
          "filename": "post-release-workflow.yaml",
          "inputs": {
            "tag": "{{tag_name}}",       // Dynamic variable substitution
            "url": "{{release_url}}",
            "environment": "production"  // Static value
          }
        }
      ]
    }
  }
}
```

#### Variable Substitution
Use `{{variable_name}}` in `inputs`.

| Variable | Description |
|----------|-------------|
| `tag_name` | The git tag (e.g., `v1.0.0`) |
| `release_url` | Link to the release on GitHub |
| `release_sha` | Commit SHA of the release |
| `path` | Path of the released package (e.g., `pkg/one`) |

#### Extending
To add more variables, update the `releaseData` object in `.github/workflows/release-please.yml`:

```javascript
const releaseData = {
  tag_name: release_info[`${path}--tag_name`],
  // ... add new properties here
};
```
