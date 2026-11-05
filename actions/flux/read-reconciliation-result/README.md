# Description

This GitHub Action reads a deployment result that Flux sends to a product repository as a `repository_dispatch` event. Use it as the first step of the workflow that receives the events. It validates the event and returns `succeeded`, `failed` or `ignored`, so the next jobs can run tests or send a message. It reads only the event file of the job, and needs no checkout, token or permissions. [RFC 0019](https://github.com/Altinn/altinn-platform/blob/main/rfcs/0019-flux-workflow-notifications.md) describes the design.

## Requirements

- The platform team has approved the repository, and the product namespace has a `githubdispatch` Provider and Alerts for it. Each Alert sets `schema_version: "1"`, `product`, `environment`, `cluster` and `repository` (`<owner>/<name>`) in `spec.eventMetadata`. The onboarding guide ([#4386](https://github.com/Altinn/altinn-platform/issues/4386)) will describe the setup.
- The application `Kustomization` has health checks: `spec.wait: true` or `spec.healthChecks`. Without health checks, Flux sends no success event.
- The workflow is on the default branch. GitHub starts `repository_dispatch` workflows only from the default branch.

## Inputs and outputs

| Input | Description |
|-------|-------------|
| `expected_sender_id` | Numeric id of the bot account of the platform GitHub App. Get it with `gh api 'users/<app slug>[bot]' --jq .id`. Do not take it from the event. |
| `product` | Product identifier. The event must have the same `metadata.product`. |
| `subscriptions` | JSON array of the accepted subscriptions. Each entry has `environment`, `cluster`, `namespace` and `kustomization`, and can have `oci_repository`. Use exact names, without wildcards. |

| Output | Description |
|--------|-------------|
| `outcome` | `succeeded`, `failed` or `ignored`. |
| `environment`, `cluster` | Values of the matching subscription. |
| `revision` | OCI artifact revision, for example `main@sha256:<digest>`. Empty when the event has no OCI revision. |
| `app_version`, `app_commit` | Release metadata of this revision, or empty. See [Release metadata](#release-metadata). |
| `event_json` | The validated Flux event as JSON. |

`succeeded` is the `Health check passed` event of the `Kustomization`. Flux sends it only when a revision changes resources or the health checks recover, so a revision that changes nothing gives no event. `failed` is an error event of the `Kustomization` or the `OCIRepository`. Flux sends one for each failed attempt, and a later attempt can still succeed. Other events are `ignored`.

The step fails, without outputs, when the event does not match the inputs or is malformed. This means a configuration error or an event from an unexpected sender, not a failed deployment. Do not set `continue-on-error: true` on the step, so that a rejected event stops the workflow.

## Usage

Pin the action to a commit. Pass the outputs to other steps with `with:` or `env:`. Do not put them in a `run:` script with `${{ }}`, because the event text is not trusted.

```yaml
on:
  repository_dispatch:
    types:
      - Kustomization/dialogporten-apps.product-dialogporten
      - OCIRepository/dialogporten-manifests.product-dialogporten

permissions: {}

jobs:
  read:
    runs-on: ubuntu-latest
    outputs:
      outcome: ${{ steps.result.outputs.outcome }}
      environment: ${{ steps.result.outputs.environment }}
    steps:
      - id: result
        uses: Altinn/altinn-platform/actions/flux/read-reconciliation-result@<commit> # main
        with:
          expected_sender_id: ${{ vars.FLUX_DISPATCH_BOT_ID }}
          product: dialogporten
          subscriptions: |
            [{"environment": "at23", "cluster": "dis-core-at23-aks", "namespace": "product-dialogporten",
              "kustomization": "dialogporten-apps", "oci_repository": "dialogporten-manifests"}]

  tests:
    needs: read
    if: needs.read.outputs.outcome == 'succeeded' && needs.read.outputs.environment == 'at23'
    uses: ./.github/workflows/e2e-tests.yml
```

Map the environment to the test target in the workflow. Do not take a URL or a credential from the event.

## Release metadata

Set these annotations on the watched `Kustomization` to get `app_version` and `app_commit`:

```yaml
metadata:
  annotations:
    event.toolkit.fluxcd.io/app_version: "1.126.0"
    event.toolkit.fluxcd.io/app_commit: "<full 40-character commit id>"
    event.toolkit.fluxcd.io/release_revision: "main@sha256:<digest>"
```

The action returns the values only when they are valid and `release_revision` is the revision of the event. Annotations from an earlier release give empty outputs.

## Limits

- Delivery is best effort. Flux does not retry a failed delivery, so an event can be lost. Keep a manual way to start the jobs.
- An event can arrive more than once, late or out of order. The action does not remove repeated events. Check the running release before and after the tests.

## Development

The action is TypeScript that Node.js 24 runs directly, without a build step or runtime dependencies. TypeScript is a development dependency that checks the types. Check the types and run the tests in this directory with Node.js 24 or later:

```sh
npm ci
npm run typecheck
npm test
```
