# AGENTS.md

## What this is
dis-console is a small Go service (plain `net/http`) that reads Flux custom
resources across all namespaces, persists a normalized snapshot in a
DIS-provisioned PostgreSQL database, and serves their deployment status (plus
status history) as a read-only JSON API. It is NOT a kubebuilder operator: there is no
controller-runtime, no CRDs, and no controller-gen/envtest toolchain. It still
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
- Run against your kubeconfig: `make run` (passes `--local`)

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
- `cmd/main.go` — flags (`--http-address`, `--poll-interval`, `--local`,
  `--db-uri`, `--db-disable-entra`), HTTP server, poller ticker, DB wiring.
- `internal/flux` — dynamic-client reader + normalize to a stable `Resource`.
- `internal/dbauth` — pgxpool builder; Entra-token `BeforeConnect` hook in the
  cluster, PGPASSWORD/trust fallback when Entra is disabled (Kind/CI/local).
- `internal/store` — pgxpool store: embedded `schema.sql`, `Migrate`, `Sync`
  (upsert + `flux_status_event` history + prune), and summary/list/get queries.
- `internal/api` — `net/http` mux + JSON handlers serving from the store.
- `test/e2e` — `e2e`-tagged store test run against Kind Postgres.
- `config/kind/postgres.yaml` — trust-auth Postgres for the e2e.
