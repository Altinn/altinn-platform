# Description

This GitHub Action reads a deployment result that Flux sends to a product repository. Use it as the first step of a workflow that starts on the `repository_dispatch` events from Flux. The next jobs of the workflow then run end-to-end tests after a success, or send a Slack message after a failure.

The DIS core cluster sends the events with the Flux [`githubdispatch` Provider](https://fluxcd.io/flux/components/notification/providers/#github-dispatch). Each event holds a Flux event about a `Kustomization` or an `OCIRepository`, and the metadata of the subscription. The action validates the event. It returns `succeeded`, `failed` or `ignored`, with the environment, the cluster, the artifact revision and the release metadata.

The action reads only the event file of the job. It needs no checkout, no GitHub token, no access to the Kubernetes API and no OIDC token. [RFC 0019](https://github.com/Altinn/altinn-platform/blob/main/rfcs/0019-flux-workflow-notifications.md) describes the design, and [#4369](https://github.com/Altinn/altinn-platform/issues/4369) tracks the rollout.

Each event starts a new workflow run. The event does not go to the workflow that published the artifact. Delivery is best effort: an event can be lost, repeated or late. See [Limits](#limits).

## Requirements

- The platform team has approved the destination repository and supplies the dispatch token for it. See [#4369](https://github.com/Altinn/altinn-platform/issues/4369).
- The application `Kustomization` has health checks: `spec.wait: true` or `spec.healthChecks`. Without health checks, Flux sends no success event.
- The product namespace has a `githubdispatch` Provider and Alerts with the subscription metadata. See [Subscription](#subscription).
- The receiving workflow is on the default branch. GitHub starts `repository_dispatch` workflows only from the default branch.

## Inputs

| Input | Required | Description |
|-------|----------|-------------|
| `expected_sender_id` | Yes | Numeric id of the bot account of the platform GitHub App that sends the events. Get it with `gh api 'users/<app slug>[bot]' --jq .id`. Set it in the workflow or in a repository variable. Do not take it from the event. |
| `product` | Yes | Product identifier, for example `dialogporten`. The event must have the same `metadata.product`. |
| `subscriptions` | Yes | JSON array of the subscriptions that the workflow accepts. Each entry has `environment`, `cluster`, `namespace` and `kustomization`. Add `oci_repository` to accept source failures. Use exact names. Wildcards are not permitted. |

## Outputs

| Output | Description |
|--------|-------------|
| `outcome` | `succeeded`, `failed` or `ignored`. See [Classification](#classification). |
| `environment` | Environment of the matching subscription. |
| `cluster` | Cluster of the matching subscription. |
| `revision` | OCI artifact revision of the event, for example `main@sha256:<digest>`. Empty when the event has none, for example after a source failure, or when the revision is not an OCI revision with a SHA-256 digest, for example `main@sha1:<commit>` from a Git source. |
| `app_version` | Application version from the release metadata of this revision. Empty when it is not available. See [Release metadata](#release-metadata). |
| `app_commit` | Full application commit from the release metadata of this revision. Empty when it is not available. |
| `event_json` | The validated Flux event as one line of JSON: the resource, `reason`, `severity`, `timestamp`, `message` and `metadata`. The action removes release metadata that does not belong to the revision. |

The action also writes the result to the job summary.

## Classification

| Flux event | Outcome |
|------------|---------|
| `Kustomization`, severity `info`, reason `Progressing`, message `Health check passed in …`. Flux 2.10 also sets the action `HealthCheck`. An empty action counts as no action. | `succeeded` |
| `Kustomization`, severity `error`, for example `HealthCheckFailed`, `BuildFailed` or `ReconciliationFailed`. | `failed` |
| `OCIRepository`, severity `error`, for example `OCIArtifactPullFailed`. Often without a revision. | `failed` |
| All other events, for example `DependencyNotReady`, `HealthCheckCanceled`, lists of changed resources and `ReconciliationSucceeded`. | `ignored` |
| An event that does not pass the [validation](#validation). | The action fails. |

kustomize-controller sends `Health check passed in …` in two cases: a new revision changed resources and passed the health checks, or the health checks pass after a failure or for the first time. A new revision that changes no resource gives no success event.

kustomize-controller sends the final `ReconciliationSucceeded` event after every reconciliation, also when nothing changed. notification-controller does not send it to `githubdispatch` ([fluxcd/flux2#3933](https://github.com/fluxcd/flux2/issues/3933)). If it arrives, the action ignores it.

The success event has no reason of its own, so the action also checks the message. [#4373](https://github.com/Altinn/altinn-platform/issues/4373) asks Flux for a stable reason. Run the tests with real events after each Flux upgrade ([#4380](https://github.com/Altinn/altinn-platform/issues/4380)).

A failure describes one attempt. Flux tries again, and a later attempt for the same revision can succeed.

## Validation

The action fails, and publishes no outputs, in these cases:

- The workflow event is not `repository_dispatch`.
- The id of the sender is not `expected_sender_id`.
- The repository of the event, or `metadata.repository`, is not the repository of the workflow.
- The resource is not a `Kustomization` or an `OCIRepository`.
- The event type is not `<kind>/<name>.<namespace>` of the involved object.
- `metadata.schema_version` is not a supported contract version. See [Versions](#versions).
- `metadata.product` is not `product`.
- The reporting controller is not `kustomize-controller` for a `Kustomization`, or not `source-controller` for an `OCIRepository`.
- No single subscription matches the environment, the cluster, the namespace and the resource. Values from different subscriptions do not match.
- A success event has no OCI revision with a SHA-256 digest.
- A field of the event is missing or has the wrong type.

A failed action means that the configuration is wrong, or that the event did not come from the subscription. It does not mean that the deployment failed. A deployment failure gives `outcome: failed`, and the step succeeds. Do not set `continue-on-error: true` on the step, and keep the next jobs dependent on it.

## Usage

Pin the action to a commit. Use the outputs through `with:` or `env:`. Do not put an output into a `run:` script with `${{ }}`: the message of an event is not trusted text.

```yaml
name: Flux reconciliation result

on:
  repository_dispatch:
    types:
      - Kustomization/dialogporten-apps.product-dialogporten
      - OCIRepository/dialogporten-manifests.product-dialogporten

permissions: {}

jobs:
  read:
    runs-on: ubuntu-latest
    permissions: {}
    outputs:
      outcome: ${{ steps.result.outputs.outcome }}
      environment: ${{ steps.result.outputs.environment }}
      revision: ${{ steps.result.outputs.revision }}
      event_json: ${{ steps.result.outputs.event_json }}
    steps:
      - id: result
        uses: Altinn/altinn-platform/actions/flux/read-reconciliation-result@<commit> # main
        with:
          expected_sender_id: ${{ vars.FLUX_DISPATCH_BOT_ID }}
          product: dialogporten
          subscriptions: |
            [
              {
                "environment": "at23",
                "cluster": "dis-core-at23-aks",
                "namespace": "product-dialogporten",
                "kustomization": "dialogporten-apps",
                "oci_repository": "dialogporten-manifests"
              }
            ]

  tests:
    needs: read
    if: needs.read.outputs.outcome == 'succeeded' && needs.read.outputs.environment == 'at23'
    uses: ./.github/workflows/deployment-succeeded.yml
    with:
      environment: at23
      expected_revision: ${{ needs.read.outputs.revision }}

  notify:
    needs: read
    if: needs.read.outputs.outcome == 'failed'
    uses: ./.github/workflows/deployment-failed.yml
    with:
      environment: ${{ needs.read.outputs.environment }}
      flux_event: ${{ needs.read.outputs.event_json }}
```

- `deployment-succeeded.yml` and `deployment-failed.yml` are examples of product workflows. A called workflow gets only the permissions that the calling job grants, so grant them on that job, for example `id-token: write` for an Azure login.
- `github.sha` is the commit of the default branch, not the deployed application.
- Map the environment of the subscription to the test target in the workflow. Do not take a URL or a credential from the event.

## Subscription

The action needs these keys in the metadata of the event. Set them in `spec.eventMetadata` of each Alert:

| Key | Value |
|-----|-------|
| `schema_version` | `"1"` |
| `product` | The `product` input of the action. |
| `environment` | The environment of the subscription. |
| `cluster` | The cluster of the subscription. |
| `repository` | The destination repository, `<owner>/<name>`. It must be the repository in the `address` of the Provider. |

Flux adds `revision` and `originRevision`. Do not set them in an Alert or an annotation.

Use one Alert with `eventSeverity: info` for the application `Kustomization`. The `info` severity includes errors. Use a second Alert with `eventSeverity: error` for the `OCIRepository`. Name each resource exactly:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Alert
metadata:
  name: dialogporten-apps-result
  namespace: product-dialogporten
spec:
  providerRef:
    name: dialogporten-github
  eventSeverity: info
  eventSources:
    - kind: Kustomization
      name: dialogporten-apps
  eventMetadata:
    schema_version: "1"
    product: dialogporten
    environment: at23
    cluster: dis-core-at23-aks
    repository: Altinn/dialogporten
```

The names are examples. The onboarding guide ([#4386](https://github.com/Altinn/altinn-platform/issues/4386)) will give the complete Provider and Alerts.

## Release metadata

The action returns `app_version` and `app_commit` only when `metadata.release_revision` is the same as the revision of the event. Set the three values as annotations on the watched `Kustomization`, not on a Deployment or an Alert:

```yaml
metadata:
  annotations:
    event.toolkit.fluxcd.io/app_version: "1.126.0"
    event.toolkit.fluxcd.io/app_commit: "<full 40-character commit id>"
    event.toolkit.fluxcd.io/release_revision: "main@sha256:<digest>"
```

An annotation can still describe the previous release when a new artifact fails. Then the revisions are different, and the outputs are empty. `app_version` must start with a letter or a digit and contain only letters, digits, `.`, `+`, `_` and `-`, at most 128 characters. `app_commit` must be a full commit id. The action removes values in another format.

A product can also map the revision itself. `metadata.originRevision` is the commit that the publisher recorded in the artifact, for example `main/<manifests commit>`. Dialogporten reads the image tag at that commit ([#4379](https://github.com/Altinn/altinn-platform/issues/4379)).

## Limits

- Delivery is best effort. notification-controller sends each event once and does not retry it. A failed request or a restart of the controller loses the event. The platform team monitors `NotificationDispatchFailed` ([#4380](https://github.com/Altinn/altinn-platform/issues/4380)). Keep a manual way to start the jobs.
- Events can arrive more than once, late or out of order. A test job must check the running release before and after the tests. The action does not remove repeated events.
- Flux sends a failure event for each failed attempt. A notification job must suppress repeated messages if it needs to.
- GitHub accepts at most 10 top-level fields in `client_payload` and less than 64 KB. A Flux 2.10 event has 10 fields. Keep annotations small.

## Versions

| `metadata.schema_version` | Flux | Success event |
|---------------------------|------|---------------|
| `1` | 2.9 (`microsoft.flux` extension 1.26) | Severity `info`, reason `Progressing`, message `Health check passed in …` |
| `1` | 2.10 | The same, with the action `HealthCheck`. The event also has `relatedObject`. |

| Environment | Product | Status |
|-------------|---------|--------|
| `at23` | Dialogporten | Pilot planned ([#4384](https://github.com/Altinn/altinn-platform/issues/4384)) |

The platform team updates these tables when the contract or the validated environments change. An incompatible change needs a new contract version.

## Tests

The tests use only Node.js 21 or later. The test workflow uses Node.js 24, like the action. Run them in this directory:

```sh
node --test "test/*.test.mjs"
```

[`test/fixtures`](test/fixtures) has `repository_dispatch` events with Flux 2.9 and Flux 2.10 payloads. The ids and digests in the fixtures are examples. The workflow [`flux-read-reconciliation-result-test.yml`](../../../.github/workflows/flux-read-reconciliation-result-test.yml) runs the tests for each change.
