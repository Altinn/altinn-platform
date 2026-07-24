# AGENTS.md

## What this is
dis-console is a small Go service (plain `net/http`) with two subcommands:
- `dis-console agent` — runs per cluster: reads Flux and DIS custom resources
  across all namespaces and persists a normalized snapshot into that cluster's
  own tenant PostgreSQL database. Exposes only `/healthz` and `/readyz` (no
  JSON API).
- `dis-console server` — runs centrally: migrates the central read model,
  incrementally syncs every tenant database on the shared server into it, and
  serves the fleet JSON API over it (`/api/clusters`, `?cluster=` filters,
  staleness) plus `/healthz` + `/readyz`.

It is NOT a kubebuilder operator: no controllers, no CRDs, and no
controller-gen/envtest toolchain. (controller-runtime is present only as an
*indirect* dependency: the typed Flux `api` packages register a scheme builder
that imports it; dis-console uses their status structs, never the framework.)
It still
includes the shared `../../Makefile.common` for the generic Go tasks, but
overrides the build/test wiring (`BUILD_DEPS`/`TEST_DEPS`/`GO_TEST_CMD`) so none
of the kubebuilder codegen/envtest machinery runs. The verification ergonomics
(the `*-cache` targets, `setup-local-env`, `run-checks-ci`) are therefore shared
with the dis-* operators.

## Quick start
- Bootstrap tooling once per fresh worktree: `make setup-local-env`
  (installs `bin/golangci-lint` pinned to the CI version).
- List targets: `make help`

## Common commands
- Format: `make fmt-cache`
- Vet: `make vet-cache`
- Lint: `make lint-cache`
- Unit tests: `make test-cache`
- Build binary: `make build-cache`
- Run the agent against your kubeconfig: `make run` (passes `agent --local`)

The `*-cache` targets run with a repo-local `GOCACHE` so checks stay
sandbox-friendly.

## Required verification for code changes
If you modify any files under `cmd/**` or `internal/**`, you MUST run this
before producing a final answer/patch:

    make run-checks-ci-cache

It runs fmt, vet, test (with coverage), and lint. Run `make setup-local-env`
first on a fresh worktree so `bin/golangci-lint` is present.

In the final response, include the command(s) you ran and whether they passed.
If you cannot run them, you MUST say so explicitly and explain why.

`make test` (and the unit suite) does NOT touch a database: the API is tested
against an in-memory fake store. The store's SQL is validated against a real
PostgreSQL by the Kind e2e:

    make test-e2e-kind-ci

It stands up a trust-auth `postgres:16` on a throwaway Kind cluster, port-forwards
it, runs the `e2e`-tagged store test (`./test/e2e`) over pgx, and tears the
cluster down. Requires `kind` + a container runtime (podman locally, docker in
CI). This is the dis-console analogue of the operators' `test-e2e-kind-ci` job.

## Non-negotiable
Do not claim checks passed unless you actually ran them.

## Layout
- `cmd/main.go` — subcommand dispatch. `agent` runs the per-cluster sweep loop
  (health probes only); `server` migrates the central schema, runs the tenant
  sync loop, and serves the fleet API. Each wires its own DB pool and flags.
- `internal/flux` — version-agnostic dynamic-client reader for the Flux and DIS
  kinds plus the GitOps-applied `apps` workloads (Deployment/StatefulSet/
  DaemonSet, filtered in Sweep on the kustomize-controller label or Helm's
  managed-by label, the latter resolved to the owning HelmRelease via the
  `meta.helm.sh` release annotations);
  `normalize.go` decodes the projected status into the typed Flux `api`
  structs (kustomize/helm/source `api` + `pkg/apis/meta`) via runtime
  conversion — for the DIS kinds into a minimal local struct (never the
  operator modules) that also projects `azureResourceId` and `parent`, and for
  the workloads into `k8s.io/api/apps/v1` structs projecting `images` and
  per-kind readiness — while keeping the full object for `raw`. `hygiene.go`
  strips volatile metadata (`managedFields`, `resourceVersion`), computes the
  `content_hash`, and caps `raw` at `MaxRawBytes`.
- `internal/dbauth` — pgxpool builder; Entra-token `BeforeConnect` hook in the
  cluster, PGPASSWORD/trust fallback when Entra is disabled (Kind/CI/local).
- `internal/store` — tenant pgxpool store: embedded `schema.sql`, `Migrate`,
  `Sync` (content-hash-gated upsert + `flux_status_event` history + prune +
  optional event-retention purge, touches the `meta` row), the `meta`
  bookkeeping table (`InitMeta`/`GetMeta`, `SchemaVersion`), and the
  server-side tenant readers (`ChangedSince`/`Keys`).
- `internal/central` — the server's central read model: cluster-keyed
  `schema.sql`, `Apply` (per-cluster upsert + prune + `cluster_report` in one
  tx), tenant `Discover`, the sync `Engine` (pulls each tenant incrementally via
  `updated_at > cursor`, ages out central events once per cycle when
  `--event-retention` is set), and the fleet-API read methods (`Clusters` with
  staleness, `Summary`/`List`/`Get`).
- `internal/health` — `/healthz` + `/readyz` handlers used by the agent.
- `internal/api` — the fleet API: `net/http` mux + JSON handlers reading the
  central store (`/api/clusters`, `?cluster=` filters, detail by cluster), wired
  by `server`. Also the base-layer views: `/api/artifacts` classifies every
  OCIRepository by its URL (product syncroot / admin syncroot / infra /
  operator — the URL is the only identity these artifacts carry) with the
  deploying Kustomizations attached, and
  `/api/kustomizations/{cluster}/{ns}/{name}/inventory` expands a
  Kustomization's applied-object set enriched with mirrored status.
- `test/e2e` — `e2e`-tagged store + central tests run against Kind Postgres.
- `config/kind/postgres.yaml` — trust-auth Postgres for the e2e.
