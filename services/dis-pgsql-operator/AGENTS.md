# AGENTS.md

## Project goals
- Provide self-service PostgreSQL provisioning for app owners via a Database CR applied through GitOps.
- Reconcile Database CRs into Azure PostgreSQL Flexible Server resources using Azure Service Operator (ASO).
- Ensure each app gets an isolated database with consistent, declarative configuration and automated lifecycle.
- Keep operations safe and observable: idempotent reconciles, clear status/conditions, and secure defaults.

## Quick start
- Install deps / tools: follow `make help` and the `Makefile` toolchain (Go, controller-gen, kustomize).
- List targets: `make help`

## Common commands
- Format: `make fmt`
- Lint: `make lint`
- Generate code: `make generate`
- Generate manifests (CRDs/RBAC/webhooks): `make manifests`
- Unit tests: `make test-ci`
- Build manager binary: `make build`

## Required verification for code changes
If you modify any files under:
- `api/**`, `cmd/**`, `internal/**`, `test/**`

You MUST run these commands before producing a final answer/patch:
1. `make fmt`
2. `make generate`
3. `make manifests` (required if `api/**` or `config/**` changed)
4. `make test-ci`

In the final response, include the command(s) you ran and whether they passed.
If you cannot run them, you MUST say so explicitly and explain why.

## Non-negotiable
Do not claim checks passed unless you actually ran them.

## CRD/API changes
If you touch `api/**`:
- Ensure `make manifests` is run (CRDs/RBAC/webhooks updated).
- If sample YAML exists (often under `config/samples/**`), try to update it to match the new schema.
- Avoid breaking changes unless explicitly intended.

## Running and deploying
- Run in Kind (local): `make test-e2e`
- Install CRDs: `make install`
- Undeploy: `make undeploy`
- Uninstall CRDs: `make uninstall`

## Git expectations
Before opening a PR, ensure:
- Never run git push by yourself
- Always suggest to create a new branch in case we are working on main by mistake
