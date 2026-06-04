# Database

`storage.dis.altinn.cloud/v1alpha1`, owned by **dis-pgsql-operator**.

Creates one PostgreSQL database inside an existing `DatabaseServer` and grants
role-based access to a set of Entra principals (app identities and/or groups).
A `Database` always needs a `DatabaseServer` to live on — create or pick one
first.

## Source of truth

- Types: `services/dis-pgsql-operator/api/v1alpha1/database_types.go`
- CRD schema: `services/dis-pgsql-operator/config/crd/bases/storage.dis.altinn.cloud_databases.yaml`
- Sample: `services/dis-pgsql-operator/config/samples/storage_v1alpha1_database.yaml`
  (shows both single-tenant and multitenant layouts)

## Spec fields

| Field | Required | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `name` | **yes** | string (1–63) | — | PostgreSQL database name; unique per server. |
| `server.name` | **yes** | string | — | Same-namespace `DatabaseServer` to host the database. |
| `access.principals` | **yes** | list (≥1) | — | Who gets access and at what role. |
| `access.principals[].role` | **yes** | enum | — | `Reader` (read-only), `Writer` (DML, no DDL), or `Owner` (DML + schema ownership). |
| `access.principals[].identityRef` | one-of | object | — | `name` of an [ApplicationIdentity](applicationidentity.md) in the same namespace. |
| `access.principals[].group` | one-of | object | — | An existing Entra group: `{name, principalId}`. |
| `deletionPolicy` | no | enum `Retain` | `Retain` | Only `Retain` is supported; the DB is kept when the resource is deleted. |

Each principal must set **exactly one** of `identityRef` or `group`. Use
`identityRef` for an app's managed identity and `group` for a human team
(e.g. DB owners).

## Template — single-tenant (own server)

```yaml
apiVersion: storage.dis.altinn.cloud/v1alpha1
kind: Database
metadata:
  name: router
  namespace: myteam-dev
spec:
  name: router
  server:
    name: db1
  access:
    principals:
      - role: Owner
        identityRef:
          name: myproduct-router-dev
      - role: Owner
        group:
          name: my-team-db-owners
          principalId: "11111111-1111-1111-1111-111111111111"
  deletionPolicy: Retain
```

## Template — multitenant (shared server)

```yaml
apiVersion: storage.dis.altinn.cloud/v1alpha1
kind: Database
metadata:
  name: orders
  namespace: myteam-dev
spec:
  name: orders
  server:
    name: shared-db
  access:
    principals:
      - role: Writer
        identityRef:
          name: myproduct-orders-dev
      - role: Owner
        group:
          name: my-team-db-owners
          principalId: "11111111-1111-1111-1111-111111111111"
  deletionPolicy: Retain
```

`server.name` must match a `DatabaseServer` in the same namespace
([DatabaseServer](databaseserver.md)), and each `identityRef.name` an
`ApplicationIdentity` in that namespace. The group `principalId` is the Entra
group object ID (quote it so YAML keeps it a string).

## Connecting an app — the connection ConfigMap

Authoring the `Database` is only half the job: the app still has to connect, and
this is the step the manifest alone does not cover. You request nothing for it —
once the Database is `Ready`, dis-pgsql-operator automatically publishes a
ConfigMap in the same namespace, **one per `access.principals[]` entry that uses
`identityRef`** (group principals get none). The workload **must** consume it, or
it has no coordinates to connect with.

- **Name (deterministic — derivable before it exists):**
  `<database.metadata.name>-<identityRef.name>-dis-pgsql`, sanitized to DNS-1123
  and hash-suffixed if it would exceed 63 chars. For the multitenant template
  above (Database `orders`, identity `myproduct-orders-dev`) →
  `orders-myproduct-orders-dev-dis-pgsql`.
- **Keys:** `host`, `port`, `dbname`, `user`, `sslmode` (always `require`), and
  `uri` (`postgresql://<user>@<host>:5432/<dbname>?sslmode=require`).
- **No password.** Auth is Entra workload identity: the pod runs as the
  ServiceAccount the app's `ApplicationIdentity` created and authenticates with a
  short-lived token, so the ConfigMap carries only non-secret coordinates. The
  `user` value is the resolved managed-identity name, which can differ from
  `identityRef.name`.

Wire the workload to it — the workload-identity ServiceAccount plus the
ConfigMap:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders
  namespace: myteam-dev
spec:
  template:
    metadata:
      labels:
        azure.workload.identity/use: "true"
    spec:
      serviceAccountName: myproduct-orders-dev
      containers:
        - name: app
          env:
            - name: DB_URI
              valueFrom:
                configMapKeyRef:
                  name: orders-myproduct-orders-dev-dis-pgsql
                  key: uri
```

`serviceAccountName` is the app's `ApplicationIdentity` name (the operator
creates a ServiceAccount of that name); the pod label switches on the
workload-identity webhook. Source of truth for the name and keys:
`services/dis-pgsql-operator/internal/connection/configmap.go`.
