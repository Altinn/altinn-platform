# AGENTS.md

## Project goals
- Provide self-service Vault resource provisioning for app owners via Vault-related CRs applied through GitOps.
- Reconcile Vault CRs into Vault resources (for example secrets, policies, roles, and bindings) with declarative lifecycle management.
- Ensure secure defaults and least-privilege access patterns across Vault integrations.
- Keep operations safe and observable: idempotent reconciles, clear status/conditions, and predictable behavior.

## RFC

- RFC reference: [RFC 0009 - Self-service Key Vault](https://github.com/Altinn/altinn-platform/blob/main/rfcs/0009-self-service-key-vault.md).

## Quick start
- Install deps / tools: follow `make help` and the `Makefile` toolchain (Go, controller-gen, kustomize) once scaffolding is present.
- List targets: `make help`

## Local workspace (dev only)
- There is no predefined module pairing yet for `dis-vault-operator`.
- Add workspace pairings only after the module dependency pattern is established.
- If you create a local `go.work`/`go.work.sum`, keep them uncommitted (dev-only artifacts).
- Avoid running `go work sync` on shared branches; it can update other modules' `go.mod`/`go.sum`.

## Common commands
- Format: `make fmt-cache`
- Lint: `make lint-cache`
- Generate code: `make generate-cache`
- Generate manifests (CRDs/RBAC/webhooks): `make manifests-cache`
- Unit tests: `make test-cache`
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

You can run all these by running `make run-checks-ci-cache`

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
- Domain logic should live in dedicated packages (for example `internal/pkg`, `internal/vault`).
- Tests for `internal/controller` code should live in `vault_controller_test.go` (Ginkgo) for high-level behavior coverage.
- All remaining tests (packages that are not controller packages) should follow standard Go unit test conventions (`x.go` + `x_test.go`).
