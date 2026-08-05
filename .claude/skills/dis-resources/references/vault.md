# Vault

`vault.dis.altinn.cloud/v1alpha1`, owned by **dis-vault-operator**.

Provisions an Azure Key Vault owned by either an `ApplicationIdentity` or a
`ServiceAccount`, optionally wired to ExternalSecrets for syncing secrets into
the namespace.

## Source of truth

- Types: `services/dis-vault-operator/api/v1alpha1/vault_types.go`
- CRD schema: `services/dis-vault-operator/config/crd/bases/vault.dis.altinn.cloud_vaults.yaml`
- Samples: `services/dis-vault-operator/config/samples/vault_v1alpha1_vault.yaml`,
  `vault_v1alpha1_service_account_vault.yaml`, `external_secrets_vault.yaml`

## Spec fields

| Field | Required | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `identityRef` | one-of | object | — | `name` of an [ApplicationIdentity](applicationidentity.md) that owns the vault. |
| `serviceAccountRef` | one-of | object | — | `name` of a ServiceAccount that owns the vault. |
| `groupObjectId` | no | string | — | Entra group object ID granted access. Must be a **lowercase** GUID. |
| `externalSecrets` | no | bool | `false` | Create a namespaced ExternalSecrets `SecretStore` for the vault. |
| `sku` | no | enum `standard`/`premium` | `standard` | Key Vault SKU. |
| `publicNetworkAccess` | no | enum `Enabled` | `Enabled` | Only `Enabled` is supported in v1. |
| `softDeleteRetentionDays` | no | int (7–90) | `90` | Soft-delete retention. |
| `purgeProtectionEnabled` | no | bool | `true` | Purge protection. |
| `tags` | no | `map[string]string` | — | Tags propagated to Azure. |

Set **exactly one** of `identityRef` or `serviceAccountRef` — the CRD rejects
both or neither.

## Template — identity-backed

```yaml
apiVersion: vault.dis.altinn.cloud/v1alpha1
kind: Vault
metadata:
  name: orders-vault
  namespace: myteam-dev
spec:
  identityRef:
    name: myproduct-orders-dev
  sku: standard
  tags:
    app: orders
    env: dev
```

## Template — service-account-backed

```yaml
apiVersion: vault.dis.altinn.cloud/v1alpha1
kind: Vault
metadata:
  name: orders-vault
  namespace: myteam-dev
spec:
  serviceAccountRef:
    name: orders-sa
  sku: standard
```

## Template — with ExternalSecrets and group access

```yaml
apiVersion: vault.dis.altinn.cloud/v1alpha1
kind: Vault
metadata:
  name: orders-vault
  namespace: myteam-dev
spec:
  identityRef:
    name: myproduct-orders-dev
  externalSecrets: true
  groupObjectId: 11111111-1111-1111-1111-111111111111
  sku: standard
```

`groupObjectId` must be lowercase; defaults (`publicNetworkAccess: Enabled`,
`softDeleteRetentionDays: 90`, `purgeProtectionEnabled: true`) are applied by
the CRD, so omit them unless overriding.
