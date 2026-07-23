# dis-console

**dis-console** is a small Go service that surfaces DIS deployment state. It
reads the live Flux custom resources and the DIS platform custom resources
(databases, vaults, identities, APIM) across **all namespaces** and serves
their deployment status as a read-only JSON API, so the question *"is
everything deployed and healthy, and if not, what failed and why?"* can be
answered without squinting at `flux get` or raw `kubectl ... -o yaml`.

> Status: moving to a fleet model. The binary has two subcommands:
> - `dis-console agent` runs in each cluster, sweeps its Flux CRs every interval,
>   and persists a normalized snapshot (plus a `flux_status_event` history table)
>   into that cluster's own tenant PostgreSQL database. It exposes only health
>   probes.
> - `dis-console server` runs centrally: it incrementally syncs the tenant
>   databases into a central read model and serves the fleet JSON API below over
>   it (`/api/clusters`, `?cluster=` filters, staleness). Status `history` in the
>   detail endpoint is the one piece still pending.

## What it reads

Azure AKS installs the upstream Flux controllers via the `microsoft.flux`
extension, so the in-cluster CRs follow the upstream schema. The Console reads
these kinds across all namespaces and resolves the served apiVersion at runtime
via a discovery `RESTMapper` (robust to Azure Flux version bumps):

| group | kind |
|---|---|
| `kustomize.toolkit.fluxcd.io` | Kustomization |
| `helm.toolkit.fluxcd.io` | HelmRelease |
| `source.toolkit.fluxcd.io` | OCIRepository, HelmRepository, HelmChart |
| `storage.dis.altinn.cloud` | DatabaseServer, Database |
| `vault.dis.altinn.cloud` | Vault |
| `application.dis.altinn.cloud` | ApplicationIdentity |
| `apim.dis.altinn.cloud` | Api, ApiVersion, Backend |
| `apps` | Deployment, StatefulSet, DaemonSet — GitOps-applied only (see below) |

A DIS kind whose CRD is not installed on a cluster is simply skipped by the
sweep, so mixed fleets keep working.

For each object it extracts a small normalized shape (`ready`/`reason`/
`message`/`revision`/`suspended`/`generation`/`observedGeneration`) from the
`Ready` condition and keeps the full object under `raw` for the detail endpoint.
Every object also projects `appliedBy` (`{name,namespace}`) from the
`kustomize.toolkit.fluxcd.io/{name,namespace}` labels — the Kustomization that
applied it — so the list endpoint can group child HelmReleases under their
parent app without fetching detail; roots and Arc-managed objects have none.
The DIS kinds add two fields the UI builds on: `azureResourceId` (the ARM id of
the provisioned Azure resource, for Portal links — from `status.resourceId`, or
the APIM ids `status.apiVersionSetID`/`status.backendID`) and `parent`
(`{kind,name}` — a Database nests under its `spec.server.name` DatabaseServer,
an ApiVersion under the Api named by its controller owner reference). The DIS
operators publish the same `Ready` condition; the APIM kinds publish only
`status.provisioningState`, which is mapped onto ready
(Succeeded→True, Failed→False, transitional→Unknown).

The `apps` workloads are mirrored only when GitOps-applied — carrying the
`kustomize.toolkit.fluxcd.io/name` label — which keeps kube-system and
Azure-managed add-ons out. They exist for one field the Flux CRs cannot
provide: `images` (`[{container,image}]` from `spec.template.spec.containers`;
init containers skipped) — the app's *effective* version. A manifest revision
or digest only names what should run, and with `postBuild` substitution the
image tag can be resolved per cluster and never exist in git. `images` rides
the list payload (`/api/resources?kind=Deployment`), so "which image runs
where" needs no per-row detail fetch. Readiness is per kind (they share no
condition semantics): Deployment takes its `Available` condition (and maps
`spec.paused` onto `suspended`); StatefulSet and DaemonSet compare
ready/desired replica counts, synthesized into a short reason/message
(`2/3 ready`).

Each kind is listed from the apiserver watch cache (`resourceVersion=0`, not a
quorum read from etcd) and the discovery cache is refreshed only periodically,
so the repeated all-namespace sweeps stay cheap; the agent polls on an interval
rather than holding a watch.

**How the fields are read.** The fetch is intentionally dynamic — the discovery
`RESTMapper` resolves whatever apiVersion Azure Flux serves, and the full object
is kept verbatim for `raw`. The projected status fields are then decoded into
the **typed Flux api structs** (`kustomize-controller/api`, `helm-controller/api`,
`source-controller/api`, and `pkg/apis/meta` for the `Ready` condition) with
`runtime.DefaultUnstructuredConverter` — typed field access without a
version-pinned typed client, and without giving up the verbatim `raw`. Trade-off:
those api packages pull `sigs.k8s.io/controller-runtime` in as an *indirect*
dependency (their scheme builders import it); dis-console uses only the status
structs, never the framework. The DIS kinds are decoded the same way but into a
minimal local struct instead of the operator api packages, which would drag the
Azure Service Operator dependency tree into this service for a handful of
fields.

## Endpoints

Both `agent` and `server` serve `/healthz` and `/readyz`. The `/api/*` endpoints
are the **fleet API** the `server` serves over the central read model; every
list/summary endpoint takes an optional `?cluster=` filter. (Status `history` in
the detail endpoint is not populated yet.)

| method | path | served by | description |
|---|---|---|---|
| GET | `/healthz` | agent + server | liveness (always 200) |
| GET | `/readyz` | agent + server | 200 once the first sweep (agent) / sync cycle (server) completes **and** the database pings |
| GET | `/api/clusters` | server | every synced cluster + sweep/sync times, counts, and a `stale` flag |
| GET | `/api/summary?cluster=` | server | counts per kind by ready state (+ suspended); fleet-wide or one cluster |
| GET | `/api/resources?cluster=&kind=&namespace=&ready=` | server | normalized rows; `ready=False` is the "what's broken" view |
| GET | `/api/kustomizations?cluster=` | server | alias for `?kind=Kustomization` |
| GET | `/api/helmreleases?cluster=` | server | alias for `?kind=HelmRelease` |
| GET | `/api/resources/{cluster}/{kind}/{namespace}/{name}` | server | single row incl. the full `raw` object (status `history` pending) |
| GET | `/api/artifacts?cluster=&class=` | server | base-layer OCI artifacts: every OCIRepository classified by URL (`product-syncroot`, `admin-syncroot`, `infra`, `operator`, `other`) with fetched revision (`tag@sha256:…`), pushed git origin, and the deploying Kustomizations |
| GET | `/api/kustomizations/{cluster}/{namespace}/{name}/inventory` | server | a Kustomization's applied-object set (Flux `status.inventory`) expanded, each entry enriched with the mirrored row when it is a swept kind |

## Database & auth

The agent upserts each Flux resource into `flux_resource` and, whenever its
`ready`/`reason`/`revision` changes, appends a row to `flux_status_event`;
objects that disappear from the cluster are pruned each sweep. The schema is
created on startup (`CREATE TABLE IF NOT EXISTS`), and a singleton `meta` row
records the schema version, agent build, and last sweep time.

Write hygiene keeps the shared server from churning at fleet scale: before
storing, the volatile metadata fields (`managedFields`, `resourceVersion`) are
stripped, a `content_hash` is computed, and the `raw` payload is rewritten only
when that hash changes (unchanged rows don't re-TOAST every sweep). One `raw`
blob is capped at 256 KiB; larger objects are stored as a compact stub.

Connection coordinates come from `--db-uri` (default `DB_URI` env) — in the
cluster this is the DIS connection ConfigMap's `uri` key, which carries no
password. Authentication:

- **In the cluster:** a workload-identity Entra token is fetched per new
  connection (`BeforeConnect`) and used as the Postgres password — no static
  secret. This is the DIS database-consumption path.
- **Kind / CI / local:** pass `--db-disable-entra` (or `DB_DISABLE_ENTRA=1`) to
  skip Entra and authenticate with `PGPASSWORD` (or trust auth), since there is
  no workload identity there.

## Run locally

Run the agent against your current kubeconfig, persisting to a local Postgres.
It sweeps the Flux CRs and serves health probes on `:8080` (the fleet API lives
in the server, which isn't built yet):

```bash
make setup-local-env   # once per fresh checkout: installs bin/golangci-lint

# a throwaway Postgres (podman); trust auth, no password
podman run -d --rm --name dis-console-pg -e POSTGRES_HOST_AUTH_METHOD=trust -p 5432:5432 postgres:16

DB_URI="postgres://postgres@localhost:5432/postgres?sslmode=disable" \
  go run ./cmd/main.go agent --local --db-disable-entra
```

```bash
curl -s localhost:8080/healthz   # liveness
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/readyz   # 200 after the first sweep
# inspect what it stored:
psql "postgres://postgres@localhost:5432/postgres?sslmode=disable" -c 'SELECT kind, namespace, name, ready FROM flux_resource ORDER BY 1,2,3;'
```

Agent flags: `--http-address` (default `:8080`), `--poll-interval` (default
`30s`), `--local` (kubeconfig instead of in-cluster config), `--db-uri` (default
`DB_URI`), `--db-disable-entra` (default `DB_DISABLE_ENTRA`),
`--event-retention` (default `720h` — delete status-history events older than
this; `0` keeps them forever). The server takes the same `--event-retention`
for the central database.

## Develop

```bash
make help                 # list targets
make run-checks-ci-cache  # fmt + vet + test + lint (the CI check suite)
make test-e2e-kind-ci     # store SQL e2e against a trust-auth Postgres on Kind
make docker-build         # build the container image (podman by default)
```

`make test`/`run-checks-ci` never touch a database (the API is tested against an
in-memory fake store); the store's SQL is validated by the Kind e2e above.

See [AGENTS.md](AGENTS.md) for the required verification flow.

## In-cluster

The fleet API is served by the `server`, deployed centrally (see the dis-console
fleet plan). Once it's deployed, reach it via port-forward:

```bash
kubectl -n product-dis port-forward svc/dis-console 8080:80
```
