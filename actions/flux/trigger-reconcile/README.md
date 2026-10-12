# Description

This GitHub Action asks Flux in a cluster to reconcile now. Use it right after a workflow publishes a Flux OCI artifact, so the cluster does not wait for the next poll of its `OCIRepository`.

The action calls a Flux [`Receiver`](https://fluxcd.io/flux/components/notification/receivers/) of type `generic-oidc` and authenticates with the job's GitHub Actions OIDC token. It needs no secret and no access to the Kubernetes API. The request only says "check now": Flux still pulls the artifact from the registry itself.

The action fails when Flux does not accept the request. Polling still picks the artifact up, so set `continue-on-error: true` on the step unless a missed trigger must fail your job.

## Requirements

- The calling job has `permissions: id-token: write`.
- The cluster runs Flux 2.9 or later (`microsoft.flux` extension 1.26.1 or later) and exposes the notification-controller webhook receiver at `url`.
- A `Receiver` of type `generic-oidc` exists in your namespace and lists the resources to reconcile.

## Inputs

| Input | Required | Description |
|-------|----------|-------------|
| `url` | Yes | Base URL of the cluster's Flux webhook receiver, for example `https://flux-webhook.at23.dis-core.altinn.cloud`. Must use `https`. |
| `receiver_name` | Yes | Name of the `Receiver`. |
| `receiver_namespace` | Yes | Namespace of the `Receiver`. |
| `audience` | No | Audience of the OIDC token. Must equal the audience the `Receiver` expects. Defaults to `url` without trailing slashes. |

## Sample

Trigger the reconcile after the step that pushes and tags the artifact:

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
        uses: Altinn/altinn-platform/actions/flux/build-push-image@main
        with:
          ...
      - name: Trigger Flux reconcile
        continue-on-error: true
        uses: Altinn/altinn-platform/actions/flux/trigger-reconcile@main
        with:
          url: https://flux-webhook.at23.dis-core.altinn.cloud
          receiver_name: my-product-manifests
          receiver_namespace: product-my-product
```

The matching `Receiver`, deployed with the product's own Flux resources:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1
kind: Receiver
metadata:
  name: my-product-manifests
  namespace: product-my-product
spec:
  type: generic-oidc
  oidcProviders:
    - issuerURL: https://token.actions.githubusercontent.com
      audience: https://flux-webhook.at23.dis-core.altinn.cloud
      validations:
        - expression: claims.repository_owner_id == '<org id>' && claims.repository_id == '<repo id>'
          message: token is not issued to the publishing repository
        - expression: claims.ref == 'refs/heads/main'
          message: token is not issued for the main branch
  resources:
    - apiVersion: source.toolkit.fluxcd.io/v1
      kind: OCIRepository
      name: my-product-manifests
```

GitHub issues OIDC tokens to every repository, so a valid signature does not authorize a request on its own. The `validations` must pin the caller. Prefer the numeric `repository_id` and `repository_owner_id` claims: a name can be reused after a rename or transfer, an id cannot.

List only the source (`OCIRepository`) under `resources`. Flux reconciles the `Kustomization` that uses it when the source reports a new revision.

## Failures

| Response | Meaning |
|----------|---------|
| `400` | Flux rejected the token. The audience or a `validations` expression does not match, or Flux could not reach the token issuer. The action prints the token's claims so you can compare them with the `Receiver`. The exact reason is in the notification-controller log. |
| `404` | No `Receiver` has this name and namespace, the `Receiver` has another type or has never been ready, or the base URL does not reach the notification-controller. |
| `500` | Flux could not request the reconcile. A resource listed in the `Receiver` does not exist, or the `Receiver` is not allowed to reference it. |
| `503` | The `Receiver` is suspended or not ready, or the notification-controller is unavailable. |

The action retries connection errors, timeouts and `408`, `429` and `5xx` responses up to three times, and stops retrying after 60 seconds.
