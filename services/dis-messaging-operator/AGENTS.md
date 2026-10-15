# AGENTS.md

## Project goals
- Provide self-service queues on Azure Service Bus for app owners via the `Queue` CR applied through GitOps.
- Reconcile `Queue` CRs into Azure Service Operator (ASO) `NamespacesQueue` and `RoleAssignment` resources in the shared Service Bus namespace of the cluster.
- Ensure safe platform defaults and keep the tenant-facing `Queue` CRD stable while the ASO API changes.
- Keep operations safe and observable: idempotent reconciles, clear status/conditions, and predictable behavior.

## RFC

- RFC reference: [RFC 0018 - Self-service messaging](../../rfcs/0018-self-service-messaging.md).

## Quick start
- Install deps / tools: `make setup-local-env`
- List targets: `make help`

## Common commands
- Format: `make fmt-cache`
- Lint: `make lint-cache`
- Generate code: `make generate-cache`
- Generate manifests (CRDs/RBAC/webhooks): `make manifests-cache`
- Unit tests: `make test-cache`
- Default test entrypoint: use `make test` / `make test-cache`, not raw `go test`, unless you are isolating a narrow debugging case.
- Build manager binary: `make build-cache`

## Required verification for code changes
If you modify any files under:
- `api/**`, `cmd/**`, `internal/**`, `test/**`, `config/**`

You MUST run these commands before producing a final answer/patch:
1. `make fmt-cache`
2. `make generate-cache`
3. `make manifests-cache` (required if `api/**` or `config/**` changed)
4. `make test-cache`
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

## Git expectations
Before opening a PR, ensure:
- Never run git push by yourself
- Always suggest to create a new branch in case we are working on main by mistake
- Do not use `git add -f` / `git add --force` when preparing PRs. If a file is ignored, treat it as local-only unless the user explicitly asks to stage it.

## PR description file
- When working on a branch and making changes, always create or update `pr_description.md` in the repository root.
- `pr_description.md` must contain an `ASCII Diagram` section.
- Do not add extra sections (for example test-delta summaries) unless explicitly requested by the user.
- Keep this file in sync as the branch evolves so it is ready to use in the PR.

## Code organization
- `internal/controller` should only contain high-level controller duties and orchestration.
- Domain logic should live in dedicated packages (for example `internal/messaging`).
- Shared logic used by more than one DIS operator belongs in `services/dis-common`, never copied between operators.
- Tests for `internal/controller` code should live in `queue_controller_test.go` (Ginkgo) for high-level behavior coverage.
- All remaining tests (packages that are not controller packages) should follow standard Go unit test conventions (`x.go` + `x_test.go`).
