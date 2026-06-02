# ApplicationIdentity

`application.dis.altinn.cloud/v1alpha1`, owned by **dis-identity-operator**.

Creates an Azure managed identity with workload-identity federation for an app.
It is the foundational DIS resource: its `status` exposes the `principalId` and
`clientId` that `DatabaseServer`, `Database`, and `Vault` resolve when they
reference it. Create it first when an app needs database access, a vault, or its
own workload identity.

## Source of truth

- Types: `services/dis-identity-operator/api/v1alpha1/applicationidentity_types.go`
- CRD schema: `services/dis-identity-operator/config/crd/bases/application.dis.altinn.cloud_applicationidentities.yaml`
- Sample: `services/dis-identity-operator/config/samples/application_v1alpha1_applicationidentity.yaml`
  — **note this sample is a `# TODO(user)` placeholder with an empty spec.**
  Take field details from the types file / CRD schema, not the sample.

## Spec fields

| Field | Required | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `azureAudiences` | no | `[]string` | `["api://AzureADTokenExchange"]` | Token audiences accepted from Azure. Leave unset unless a consumer needs a custom audience. |
| `tags` | no | `map[string]string` | `{}` | Tags propagated to the Azure identities created for this resource. |

Both fields are optional, so the minimal spec is effectively empty. The
identity's name (`metadata.name`) is what other resources reference via
`identityRef.name`, so name it for the app/component it serves.

## Template — minimal

```yaml
apiVersion: application.dis.altinn.cloud/v1alpha1
kind: ApplicationIdentity
metadata:
  name: myproduct-orders-dev
  namespace: myteam-dev
spec: {}
```

## Template — with tags

```yaml
apiVersion: application.dis.altinn.cloud/v1alpha1
kind: ApplicationIdentity
metadata:
  name: myproduct-orders-dev
  namespace: myteam-dev
spec:
  tags:
    app: orders
    env: dev
```

Replace `myproduct-orders-dev` with a name that identifies the app/component and
environment, and `myteam-dev` with the team's namespace.
