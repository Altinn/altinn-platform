# DatabaseServer

`storage.dis.altinn.cloud/v1alpha1`, owned by **dis-pgsql-operator**.

Provisions an Azure PostgreSQL Flexible Server. A `DatabaseServer` hosts one or
more `Database` resources; it does not create app databases itself. Two modes:

- **Dedicated** (default) — a server for a single app/team. Omit `network`.
- **Shared** — a server that hosts many apps' databases. Requires `network`
  pointing at a pre-existing delegated subnet + private DNS zone.

`mode` is **immutable** after creation.

## Source of truth

- Types: `services/dis-pgsql-operator/api/v1alpha1/databaseserver_types.go`
- CRD schema: `services/dis-pgsql-operator/config/crd/bases/storage.dis.altinn.cloud_databaseservers.yaml`
- Sample: `services/dis-pgsql-operator/config/samples/storage_v1alpha1_databaseserver.yaml`

## Spec fields

| Field | Required | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `version` | **yes** | int (≥9) | — | PostgreSQL major version, e.g. `16`, `17`. |
| `serverType` | **yes** | string | — | Size/profile, e.g. `dev`, `prod`. Drives HA/backup defaults. |
| `auth.admin.identity` | **yes** | IdentitySource | — | Either `identityRef.name`, **or** both `name` + `principalId` — not a mix. |
| `auth.admin.serviceAccountName` | no | string | `identityRef.name` | ServiceAccount used for workload identity when provisioning child Database access. |
| `mode` | no | enum `Dedicated`/`Shared` | `Dedicated` | Immutable. `Shared` requires `network`. |
| `network` | for Shared | object | — | Required iff `mode: Shared`; must be omitted for Dedicated. |
| `network.delegatedSubnetResourceId` | for Shared | string | — | ARM ID of an existing delegated subnet. |
| `network.privateDnsZoneResourceId` | for Shared | string | — | ARM ID of an existing private DNS zone. |
| `storage.sizeGB` | no | int32 | operator default | Initial storage size. |
| `storage.tier` | no | string | operator default | Performance tier, e.g. `P10`. |
| `highAvailabilityEnabled` | no | bool | `true` for prod, else `false` | Zone-redundant HA. |
| `backupRetentionDays` | no | int (7–35) | `30` prod / `14` non-prod | Backup retention. |
| `enableExtensions` | no | enum set | — | Subset of `hstore`, `pg_cron`, `pg_stat_statements`, `pgaudit`, `uuid-ossp`. |
| `serverParams` | no | list `{name,value}` | — | PostgreSQL params. `azure.extensions`, `shared_preload_libraries`, `pgbouncer.*`, and `max_connections` are managed by the operator and rejected here. |

## Template — dedicated server

```yaml
apiVersion: storage.dis.altinn.cloud/v1alpha1
kind: DatabaseServer
metadata:
  name: db1
  namespace: myteam-dev
spec:
  version: 16
  serverType: prod
  highAvailabilityEnabled: true
  backupRetentionDays: 30
  storage:
    sizeGB: 128
    tier: P10
  auth:
    admin:
      identity:
        identityRef:
          name: db-admin
```

## Template — shared server

```yaml
apiVersion: storage.dis.altinn.cloud/v1alpha1
kind: DatabaseServer
metadata:
  name: shared-db
  namespace: myteam-dev
spec:
  mode: Shared
  version: 17
  serverType: prod
  network:
    delegatedSubnetResourceId: /subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-dis-admin-network/providers/Microsoft.Network/virtualNetworks/vnet-dis-admin-dbs/subnets/snet-postgres-shared
    privateDnsZoneResourceId: /subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-dis-admin-network/providers/Microsoft.Network/privateDnsZones/shared.private.postgres.database.azure.com
  auth:
    admin:
      identity:
        identityRef:
          name: db-admin
```

The admin `identityRef` points at an [ApplicationIdentity](applicationidentity.md)
in the same namespace. The network ARM IDs in the shared template are
placeholders — use the real subnet and private DNS zone IDs for the
environment.
