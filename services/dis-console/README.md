# dis-console

**dis-console** is a small Go service that surfaces DIS deployment state. Its
first job is Flux: it reads the live Flux custom resources across **all
namespaces** and serves their deployment status as a read-only JSON API, so the
question *"is everything deployed and healthy, and if not, what failed and
why?"* can be answered without squinting at `flux get` or raw `kubectl ... -o
yaml`.

The name is deliberately product-level (not `flux-*`) so the Console can grow
to cover other DIS state later.

> Status: PoC. A background poller sweeps the Flux CRs every interval and
> persists a normalized snapshot into a DIS-provisioned PostgreSQL database
> (with a `flux_status_event` history table); the API serves from the database.
> The in-cluster deployment lands in a follow-up change.

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

## Endpoints

| method | path | description |
|---|---|---|
| GET | `/healthz` | liveness (always 200) |
| GET | `/readyz` | 200 once the first sweep has been persisted **and** the database pings |
| GET | `/api/summary` | counts per kind by ready state (+ suspended) |
| GET | `/api/resources?kind=&namespace=&ready=` | normalized rows; `ready=False` is the "what's broken" view |
| GET | `/api/kustomizations` | alias for `?kind=Kustomization` |
| GET | `/api/helmreleases` | alias for `?kind=HelmRelease` |
| GET | `/api/resources/{kind}/{namespace}/{name}` | single row incl. the full `raw` object + recent status `history` |

## Database & auth

The poller upserts each Flux resource into `flux_resource` and, whenever its
`ready`/`reason`/`revision` changes, appends a row to `flux_status_event`;
objects that disappear from the cluster are pruned each sweep. The schema is
created on startup (`CREATE TABLE IF NOT EXISTS`).

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

Reads Flux CRs through your current kubeconfig, persists to a local Postgres,
and serves on `:8080`:

```bash
make setup-local-env   # once per fresh checkout: installs bin/golangci-lint

# a throwaway Postgres (podman); trust auth, no password
podman run -d --rm --name dis-console-pg -e POSTGRES_HOST_AUTH_METHOD=trust -p 5432:5432 postgres:16

DB_URI="postgres://postgres@localhost:5432/postgres?sslmode=disable" \
  go run ./cmd/main.go --local --db-disable-entra
```

```bash
curl -s localhost:8080/api/summary | jq
curl -s 'localhost:8080/api/resources?ready=False' | jq   # what's broken
curl -s localhost:8080/api/kustomizations | jq
```

Flags: `--http-address` (default `:8080`), `--poll-interval` (default `30s`),
`--local` (kubeconfig instead of in-cluster config), `--db-uri` (default
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

Once deployed (follow-up change), reach it via port-forward:

```bash
kubectl -n product-dis port-forward svc/dis-console 8080:80
```
