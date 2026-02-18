# AGENTS.md

## Project goals
- Provide self-service PostgreSQL provisioning for app owners via a Database CR applied through GitOps.
- Reconcile Database CRs into Azure PostgreSQL Flexible Server resources using Azure Service Operator (ASO).
- Ensure each app gets an isolated database with consistent, declarative configuration and automated lifecycle.
- Keep operations safe and observable: idempotent reconciles, clear status/conditions, and secure defaults.

## Quick start
- Install deps / tools: follow `make help` and the `Makefile` toolchain (Go, controller-gen, kustomize).
- List targets: `make help`

## Local workspace (dev only)
- To work across `dis-pgsql-operator` and `dis-identity-operator`, run `make workspace` (creates a local `go.work` at the repo root).
- `go.work` and `go.work.sum` are gitignored and should not be committed.
- To clean up, run `make workspace-clean`.
- Avoid running `go work sync` on shared branches; it can update other modules' `go.mod`/`go.sum`.

## Common commands
- Format: `make fmt-cache`
- Lint: `make lint-cache`
- Generate code: `make generate-cache`
- Generate manifests (CRDs/RBAC/webhooks): `make manifests-cache`
- Unit tests: `make test-cache`
- Build manager binary: `make build-cache`

## Required verification for code changes
If you modify any files under:
- `api/**`, `cmd/**`, `internal/**`, `test/**`

You MUST run these commands before producing a final answer/patch:
1. `make fmt-cache`
2. `make generate-cache`
3. `make manifests-cache` (required if `api/**` or `config/**` changed)
4. `make test-ci-cache`

You can run all these by running `make run-checks-ci`

In the final response, include the command(s) you ran and whether they passed.
If you cannot run them, you MUST say so explicitly and explain why.

## Non-negotiable
Do not claim checks passed unless you actually ran them.

## CRD/API changes
If you touch `api/**`:
- Ensure `make manifests-cache` is run (CRDs/RBAC/webhooks updated).
- If sample YAML exists (often under `config/samples/**`), try to update it to match the new schema.
- Avoid breaking changes unless explicitly intended.

## Running and deploying
- Run in Kind (local): `make test-e2e`
- Install CRDs: `make install-cache`
- Undeploy: `make undeploy-cache`
- Uninstall CRDs: `make uninstall-cache`

## Git expectations
Before opening a PR, ensure:
- Never run git push by yourself
- Always suggest to create a new branch in case we are working on main by mistake

## Code organization
- internal/controller should only contain high level code that reflects the controller duties and operation, all other code needs to go in another pkg, e.g see internal/pkg, internal/database for reference
- Tests for internal/controller code must go in database_controller_test.go (gingko), and these are aimed for high level testing of capabilities
- All remaining tests (pkgs that aren't controller) should follow the standard unit test go practice, ie. mycode.go + mycode_test.go
