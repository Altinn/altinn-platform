# Description

This GitHub Action asks Flux to reconcile now. Use it right after a workflow publishes a Flux OCI artifact, so the cluster does not wait for the next poll of its `OCIRepository`.

The action sends a reconcile request to the webhook endpoint of the environment on the DIS edge gateway. The edge gateway verifies the token, limits the request rate and forwards the request to the DIS core cluster. There, a Flux [`Receiver`](https://fluxcd.io/flux/components/notification/receivers/) of type `generic-oidc` verifies the token again and asks Flux to reconcile the resources that it names.

The action authenticates with the job's GitHub Actions OIDC token. It needs no secret and no access to the Kubernetes API. The request only says "check now": Flux still pulls the artifact from the registry itself. [RFC 0017](https://github.com/Altinn/altinn-platform/blob/main/rfcs/0017-on-demand-flux-reconcile.md) describes the design.

The reconcile request is an optimization. Polling continues, and the next poll fetches the artifact if the request fails. The action fails when the request fails, so set `continue-on-error: true` on the step. A failed reconcile request must not stop a deployment. The action does not report the deployment result.

## Requirements

- The webhook endpoint of the environment is enabled. See [Webhook endpoints](#webhook-endpoints).
- The DIS core cluster runs Flux 2.9 or later, which is the `microsoft.flux` extension 1.26.2 or later. Earlier versions do not have the `generic-oidc` type, and the API server rejects the `Receiver`.
- A `Receiver` of type `generic-oidc` is in your namespace. See [Receiver](#receiver).
- The calling job has `permissions: id-token: write`.
- The workflow runs in a repository in the `Altinn` GitHub organization. The edge gateway admits only this organization.

## Inputs

| Input | Required | Description |
|-------|----------|-------------|
| `url` | Yes | Webhook endpoint of the environment, `https://ops.<environment>.dis.altinn.cloud/flux-webhook`. See [Webhook endpoints](#webhook-endpoints). The action also uses it, without trailing slashes, as the audience of the OIDC token. Must use `https`, with no whitespace, query or fragment. The action writes a warning if it does not end in `/flux-webhook`. |
| `receiver_name` | Yes | Name of the `Receiver`. |
| `receiver_namespace` | Yes | Namespace of the `Receiver`. |

## Webhook endpoints

| Environment | `url` | Status |
|-------------|-------|--------|
| `at22` | `https://ops.at22.dis.altinn.cloud/flux-webhook` | Enabled |
| `at23` | `https://ops.at23.dis.altinn.cloud/flux-webhook` | Enabled |
| `yt01` | `https://ops.yt01.dis.altinn.cloud/flux-webhook` | Enabled |
| `tt02` | `https://ops.tt02.dis.altinn.cloud/flux-webhook` | Not enabled yet ([#4253](https://github.com/Altinn/altinn-platform/issues/4253)) |
| `prod` | `https://ops.prod.dis.altinn.cloud/flux-webhook` | Not enabled yet ([#4254](https://github.com/Altinn/altinn-platform/issues/4254)) |

The platform team enables the endpoint for each environment and then updates this table. In an enabled environment, the edge gateway and the DIS core cluster have the routes, and the DIS core cluster runs Flux 2.9. [#4237](https://github.com/Altinn/altinn-platform/issues/4237) tracks the rollout.

Use only the environments with the status Enabled. In the other environments, the edge gateway answers `404`. On a DIS core cluster without Flux 2.9, the API server also rejects the `Receiver`, and the product syncroot fails.

Send the reconcile request only to the endpoint of the environment that the workflow publishes. The endpoint is also the audience of the token, so a token for one environment is not valid in a different environment.

## Usage

Call the action after the step that pushes and tags the artifact. Pin the action to a commit:

```yaml
jobs:
  publish:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write
    steps:
      ...
      - name: Build and push app manifests artifact
        uses: Altinn/altinn-platform/actions/flux/build-push-image@<commit> # main
        with:
          ...
      - name: Trigger Flux reconcile
        continue-on-error: true
        uses: Altinn/altinn-platform/actions/flux/trigger-reconcile@<commit> # main
        with:
          url: https://ops.at23.dis.altinn.cloud/flux-webhook
          receiver_name: example-manifests
          receiver_namespace: product-example
```

## Receiver

Add a `Receiver` to the product syncroot of each enabled environment, next to the `OCIRepository` of the product. The audience is different in each environment, so put the `Receiver` in the overlay of the environment:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: example-manifests
  namespace: product-example
spec:
  type: generic-oidc
  oidcProviders:
    - issuerURL: https://token.actions.githubusercontent.com
      audience: https://ops.at23.dis.altinn.cloud/flux-webhook
      validations:
        - expression: claims.repository_owner_id == '5949420' && claims.repository_id == '<repository id>'
          message: token is not issued to the publishing repository
        - expression: claims.ref == 'refs/heads/main'
          message: token is not issued for the main branch
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: OCIRepository
      name: example-manifests
```

- Use the webhook endpoint of the environment as the `audience`, the same value as the `url` of the action. The edge gateway accepts only this audience.
- GitHub issues OIDC tokens to every repository, and the edge gateway admits every repository in `Altinn`. The `validations` must identify the publishing repository and the branch.
- Use the numeric ids. `5949420` is the id of the `Altinn` organization. Get the id of your repository with `gh api repos/Altinn/<repository> --jq .id`. A name can be reused after a rename or transfer, an id cannot.
- List only the source (`OCIRepository`) under `resources`. Flux reconciles the `Kustomization` that uses it when the source reports a new revision.

## Example

Dialogporten uses the action in at23. See its [`Receiver`](https://github.com/Altinn/dialogporten-manifests/blob/main/flux/syncroot/at23/dialogporten-receiver.yaml) and the step `Trigger Flux reconcile` in its [publishing workflow](https://github.com/Altinn/dialogporten-manifests/blob/main/.github/workflows/publish-flux-artifacts.yml). The step runs only for the environments in a list, so the workflow does not send requests to an environment that is not enabled.

## Failures

The action fails for every response except `200`. Its error names the source of the response, and it prints the first 2,000 bytes of the response body. On `400`, `401` and `403` it also prints the non-secret claims of the token.

| Response | Source | Meaning | Correction |
|----------|--------|---------|------------|
| `400` | Flux | Flux rejected the token, or Flux could not get the provider configuration or the signing keys from GitHub. The notification-controller log has the exact reason. | Compare the audience and the `validations` of the `Receiver` with the claims of the token. If they match, examine the connection from the notification-controller to GitHub. |
| `401` | Edge gateway | The token is missing, not valid or expired, or the edge gateway could not get the signing keys from GitHub. | Run the workflow again. If the error continues, tell the platform team. |
| `403` | Edge gateway | The body names the cause. `Audiences in Jwt are not allowed`: the audience of the token is not the webhook endpoint of the environment. `RBAC: access denied`: the token does not identify the `Altinn` GitHub organization. | Use the webhook endpoint exactly as [Webhook endpoints](#webhook-endpoints) lists it. Run the publishing workflow from a repository in `Altinn`. |
| `404` | Flux or edge gateway | Flux: no `Receiver` has this name and namespace, or the `Receiver` has never been ready. Flux serves the path only after the `Receiver` has reconciled. Edge gateway: the webhook endpoint is not enabled in this environment. Both send no response body. | Make sure that the environment is enabled. See [Webhook endpoints](#webhook-endpoints). Then examine the name, the namespace, `url`, and the status of the `Receiver`. |
| `429` | Edge gateway | The request rate of the repository or of the environment is more than its limit. | None. The action tries again, and the next poll fetches the artifact. |
| `500` | Flux | Flux could not request the reconcile. A resource that the `Receiver` lists does not exist, or the `Receiver` is not allowed to reference it. | Examine the `resources` of the `Receiver`. |
| `502`, `504` | Edge gateway | The edge gateway could not reach Flux in the DIS core cluster. | None. The next poll fetches the artifact. If the error continues, tell the platform team. |
| `503` | Flux or edge gateway | Flux: the `Receiver` is suspended or not ready. Flux sends no response body. Edge gateway: it could not reach the DIS core cluster. | Examine the status of the `Receiver`. If it is ready, the next poll fetches the artifact. |
| No response | Network | The runner could not connect to the edge gateway, or the request timed out. The action prints the curl error. | None. The next poll fetches the artifact. |

## Retries

The action tries the reconcile request again after these failures:

- `429`: the edge gateway counts the requests in each minute, so a short wait mostly gets a `429` again. The action waits as long as the `Retry-After` header or the largest `X-RateLimit-Reset` header of the response asks, at least 5 and at most 30 seconds. A `429` from the edge gateway has two `X-RateLimit-Reset` headers, and the one for the local limit of the gateway is `0`. Without such a header, it waits 30 seconds. It starts no retry later than 2 minutes after the first request.
- A connection error, a timeout, `408`, `500`, `502`, `503` and `504`: at most 3 retries, 5 seconds apart, and none later than 1 minute after the first request.

It does not try `400`, `401`, `403` and `404` again, because the same request gets the same answer. Each request can take 30 seconds, so the retries end at most about 2.5 minutes after the first request. The request for the OIDC token has its own retries, which stop after 1 minute.
