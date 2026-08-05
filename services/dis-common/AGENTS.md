# AGENTS.md

## Project goals
- Shared Go library for the DIS operators and services in this monorepo.
- Keep it small and dependency-free where possible: packages here define cross-service contracts (for example the finops tag set), so changes must stay deliberate and backwards compatible.

## How it is consumed
- Consumers depend on `github.com/Altinn/altinn-platform/services/dis-common` pinned to a pseudo-version, like the operators depend on each other.
- No `replace` directives: operator container builds copy only their own service directory and fetch dependencies from the module proxy.
- A change here only reaches an operator when its `go.mod` is bumped to a newer pseudo-version.

## Required verification for code changes
If you modify any Go file, you MUST run before producing a final answer/patch:

1. `make test-cache`
2. `make lint-cache`

You can run both with `make run-checks-ci-cache`.

In the final response, include the command(s) you ran and whether they passed.
If you cannot run them, you MUST say so explicitly and explain why.

## Non-negotiable
Do not claim checks passed unless you actually ran them.
