# dis-console

**dis-console** is a small Go service that surfaces DIS deployment state. Its
first job is Flux: it reads the live Flux custom resources across **all
namespaces** and serves their deployment status as a read-only JSON API, so the
question *"is everything deployed and healthy, and if not, what failed and
why?"* can be answered without squinting at `flux get` or raw `kubectl ... -o
yaml`.

The name is deliberately product-level (not `flux-*`) so the Console can grow
to cover other DIS state later.

> Status: moving to a fleet model. The binary has two subcommands:
> - `dis-console agent` runs in each cluster, sweeps its Flux CRs every interval,
>   and persists a normalized snapshot (plus a `flux_status_event` history table)
>   into that cluster's own tenant PostgreSQL database. It exposes only health
>   probes.
> - `dis-console server` runs centrally: it syncs the tenant databases into a
>   central read model and serves the fleet JSON API below. **Not built yet** —
>   it's a placeholder; the endpoints table is the target contract.

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

For each object it extracts a small normalized shape (`ready`/`reason`/
`message`/`revision`/`suspended`/`generation`/`observedGeneration`) from the
`Ready` condition and keeps the full object under `raw` for the detail endpoint.

**How the fields are read.** The fetch is intentionally dynamic — the discovery
`RESTMapper` resolves whatever apiVersion Azure Flux serves, and the full object
is kept verbatim for `raw`. The projected status fields are then decoded into
the **typed Flux api structs** (`kustomize-controller/api`, `helm-controller/api`,
`source-controller/api`, and `pkg/apis/meta` for the `Ready` condition) with
`runtime.DefaultUnstructuredConverter` — typed field access without a
version-pinned typed client, and without giving up the verbatim `raw`. Trade-off:
those api packages pull `sigs.k8s.io/controller-runtime` in as an *indirect*
dependency (their scheme builders import it); dis-console uses only the status
structs, never the framework.

## Endpoints

The `agent` serves only `/healthz` and `/readyz`. The `/api/*` endpoints are the
**fleet API** the `server` will serve over the central read model (not built
yet); they are listed here as the target contract.

| method | path | served by | description |
|---|---|---|---|
| GET | `/healthz` | agent (server planned) | liveness (always 200) |
| GET | `/readyz` | agent (server planned) | 200 once the first sweep has been persisted **and** the database pings |
| GET | `/api/summary` | server | counts per kind by ready state (+ suspended) |
| GET | `/api/resources?kind=&namespace=&ready=` | server | normalized rows; `ready=False` is the "what's broken" view |
| GET | `/api/kustomizations` | server | alias for `?kind=Kustomization` |
| GET | `/api/helmreleases` | server | alias for `?kind=HelmRelease` |
| GET | `/api/resources/{kind}/{namespace}/{name}` | server | single row incl. the full `raw` object + recent status `history` |

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
`DB_URI`), `--db-disable-entra` (default `DB_DISABLE_ENTRA`).

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
