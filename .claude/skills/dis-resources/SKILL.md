---
name: dis-resources
description: >-
  Create and configure Altinn DIS platform custom resources as GitOps-ready
  Kubernetes manifests — PostgreSQL Database and DatabaseServer, Azure Key
  Vault, and ApplicationIdentity. Use this whenever the user wants to provision
  or wire up DIS infrastructure for an Altinn app: creating a database, standing
  up a postgres server (dedicated or shared), requesting a key vault or secret
  store, or creating an application / managed identity — even when they don't
  name the CRD or the operator. Do NOT use this for developing the dis-*
  operators themselves (controller/reconciler code, regenerating CRD manifests).
---

# Authoring Altinn DIS resources

DIS ("Declarative Infrastructure Services") is a set of Kubernetes operators
that reconcile custom resources into Azure infrastructure. App teams get
self-service infra by committing a manifest to GitOps — the operator does the
Azure provisioning and reports back through the resource's `status`.

Your job with this skill is to produce **correct, minimal, GitOps-ready
manifests** for these resources, getting the cross-resource wiring and the
cross-field validation rules right the first time. The schemas live in this
repo, so prefer reading them over guessing.

## Resource catalog

| Kind | apiVersion | Operator | Purpose | Reference |
| --- | --- | --- | --- | --- |
| ApplicationIdentity | `application.dis.altinn.cloud/v1alpha1` | dis-identity-operator | Azure managed identity + workload-identity federation for an app | [references/applicationidentity.md](references/applicationidentity.md) |
| DatabaseServer | `storage.dis.altinn.cloud/v1alpha1` | dis-pgsql-operator | Azure PostgreSQL Flexible Server (dedicated or shared) | [references/databaseserver.md](references/databaseserver.md) |
| Database | `storage.dis.altinn.cloud/v1alpha1` | dis-pgsql-operator | A PostgreSQL database on a server, with role-based access | [references/database.md](references/database.md) |
| Vault | `vault.dis.altinn.cloud/v1alpha1` | dis-vault-operator | Azure Key Vault owned by an identity or service account | [references/vault.md](references/vault.md) |

## Dependency graph

`ApplicationIdentity` is the foundation: its `status` exposes the
`principalId`/`clientId` that the other resources resolve. Most useful setups
need an identity first, then the resource that references it.

```text
ApplicationIdentity
   ├─ DatabaseServer.spec.auth.admin.identity.identityRef   (server admin)
   ├─ Database.spec.access.principals[].identityRef          (app access)
   └─ Vault.spec.identityRef                                 (vault owner)

DatabaseServer  ──<  Database.spec.server.name   (a Database lives on a server)
```

All references are **by name within the same namespace**. There is no
cross-namespace referencing.

## Decision guide

Translate the request into the set of resources to author, in order:

- **"A database for one app"** → an `ApplicationIdentity` for the app + a
  `DatabaseServer` (if the team has none yet) + a `Database` that references
  both. Giving the app its own server is the *single-tenant* layout. The
  operator then publishes a connection ConfigMap the app's workload consumes —
  always tell the user how to connect (see [references/database.md](references/database.md)).
- **"Databases for several apps that share infrastructure"** → one
  `DatabaseServer` with `mode: Shared` (plus `network`) + one `Database` per app,
  each with its own `access.principals`. This is the *multitenant* layout.
- **"A secret store / key vault"** → an `ApplicationIdentity` (if the app has
  none) + a `Vault` that references it (or an existing `ServiceAccount`).
- **"Just an identity / workload identity"** → an `ApplicationIdentity` alone.

If an identity, server, or service account the user names already exists in
their repo, reference it rather than recreating it — ask if you're unsure.

## Authoring workflow

1. Use the decision guide to list the resources needed and the order to apply
   them.
2. Open the matching reference file(s) for each Kind.
3. **Read the live sample(s)** the reference points to before finalizing — the
   in-repo `config/samples/*.yaml` and `config/crd/bases/*.yaml` are the source
   of truth and may have moved past these notes.
4. Copy the template, fill in real values, and keep cross-resource names
   consistent and in a single namespace.
5. Validate (below).
6. Place the manifest at the team's GitOps path for that namespace.
7. For a `Database`, the app's connection details are published by the operator
   as a ConfigMap, not set in the manifest — surface its deterministic name and
   keys and give the user the workload snippet that consumes it (see
   [references/database.md](references/database.md)).

## Validation

- **Best (with a cluster):** from the operator directory run `make
  install-cache` to install the CRDs, then
  `kubectl apply --dry-run=server -f <file>.yaml`. Server-side dry-run enforces
  enums, ranges, required fields, and the cross-field (`XValidation`) rules.
- **Offline:** check field names, enums, and ranges against the CRD schema at
  `services/<operator>/config/crd/bases/<group>_<plural>.yaml` (the
  `openAPIV3Schema` block). Watch these cross-field rules in particular — they
  are the easiest things to get wrong:
  - **Vault:** exactly one of `identityRef` / `serviceAccountRef`.
  - **DatabaseServer:** `network` is required when `mode: Shared` and must be
    omitted when `mode: Dedicated`; `mode` is immutable once created.
  - **Database access principal:** exactly one of `identityRef` / `group`.
  - **Server admin identity:** either `identityRef`, or both `name` and
    `principalId` — not a mix.

## Conventions

- Write manifests as **pure YAML with no inline comments** — rationale belongs
  in the PR description, not the file.
- Always set `metadata.namespace` explicitly; every cross-resource reference
  resolves within that namespace.
- Use the same name for a resource and the references that point at it (e.g. an
  app's `ApplicationIdentity` name and the `identityRef.name` that uses it), so
  the wiring is obvious.

## Where these are heading

These CRDs are the low-level building blocks. A higher-level `DisApp` resource
(via kro) is planned to compose them into a single app abstraction, so treat the
names and labels you set here as a stable contract other tooling will depend on.
