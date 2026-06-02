# AGENTS.md

## What this is
dis-console is a small Go service (plain `net/http`) that reads Flux custom
resources across all namespaces and serves their deployment status as a
read-only JSON API. It is NOT a kubebuilder operator: there is no
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

## Non-negotiable
Do not claim checks passed unless you actually ran them.

## Layout
- `cmd/main.go` — flags, HTTP server, poller ticker.
- `internal/flux` — dynamic-client reader + normalize to a stable `Resource`.
- `internal/api` — `net/http` mux + JSON handlers serving the in-memory snapshot.
