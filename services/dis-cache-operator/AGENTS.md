# AGENTS.md

## Project goals
- Provide self-service Azure Managed Redis (`Microsoft.Cache/redisEnterprise`) provisioning for app owners via the `Redis` CR applied through GitOps.
- Reconcile `Redis` CRs into a Redis Enterprise cluster + database with private endpoint networking and Entra-only data-plane access.
- Ensure secure defaults: private endpoint only, TLS enforced, access keys disabled, deterministic naming.
- Keep operations safe and observable: idempotent reconciles, clear status/conditions, and predictable behavior.

## RFC

- RFC reference: [RFC 0014 - Self-service Managed Redis](https://github.com/Altinn/altinn-platform/blob/main/rfcs/0014-self-service-managed-redis.md).

## Quick start
- Install deps / tools: follow `make help` and the `Makefile` toolchain (Go, controller-gen, kustomize).
- List targets: `make help`

## Local workspace (dev only)
- There is no predefined module pairing yet for `dis-cache-operator`.
- Add workspace pairings only after the module dependency pattern is established.
- If you create a local `go.work`/`go.work.sum`, keep them uncommitted (dev-only artifacts).
- Avoid running `go work sync` on shared branches; it can update other modules' `go.mod`/`go.sum`.

## Common commands
- Format: `make fmt-cache`
- Lint: `make lint-cache`
- Generate code: `make generate-cache`
- Generate manifests (CRDs/RBAC/webhooks): `make manifests-cache`
- Unit tests: `make test-cache`
- Default test entrypoint: use `make test` / `make test-cache`, not raw `go test`, unless you are isolating a narrow debugging case.
- Build manager binary: `make build-cache`
- Vulnerability scan: `make govulncheck-cache`

## Required verification for code changes
If you modify any files under:
- `api/**`, `cmd/**`, `internal/**`, `test/**`, `config/**`

You MUST run these commands before producing a final answer/patch:
1. `make fmt-cache`
2. `make generate-cache`
3. `make manifests-cache` (required if `api/**` or `config/**` changed)
4. `make test-ci-cache`
5. `make lint-cache`

You can run all these by running `make run-checks-ci-cache`

Use the Make targets above as the primary verification path.
Do not substitute ad hoc `go test` commands for the required Make targets.
Direct `go test` is acceptable only for narrow debugging during development, and does not replace final verification.

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
- Do not use `git add -f` / `git add --force` when preparing PRs. If a file is ignored, treat it as local-only unless the user explicitly asks to stage it.

## PR description file
- When working on a branch and making changes, always create or update `pr_description.md` in the repository root.
- `pr_description.md` must contain:
  1. `Feature Behavior (BDD)`
  2. `ASCII Diagram`
- The BDD section must be based on implemented behavior and use explicit BDD keywords highlighted in text:
  - `**Given**`
  - `**When**`
  - `**Then**`
  - `**And**`
- Do not add extra sections (for example test-delta summaries) unless explicitly requested by the user.
- Keep this file in sync as the branch evolves so it is ready to use in the PR.

## Code organization
- `internal/controller` should only contain high-level controller duties and orchestration.
- Domain logic should live in dedicated packages (for example `internal/redis`).
- Role-/access-policy reconciliation helpers in `internal/controller` should live in `redis_controller_role.go`.
- Private endpoint / DNS reconciliation helpers in `internal/controller` should live in `redis_controller_network.go`.
- Tests for `internal/controller` code should live in `redis_controller_test.go` (Ginkgo) for high-level behavior coverage.
- All remaining tests (packages that are not controller packages) should follow standard Go unit test conventions (`x.go` + `x_test.go`).
