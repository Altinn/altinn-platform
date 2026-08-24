# AGENTS.md

## Project goals
- Receive Flux `notification-controller` webhooks and turn them into GitHub `repository_dispatch` events, so product teams get a deploy-finished (or deploy-failed) signal without running a receiver themselves.
- Stdlib-first: the only direct dependencies are `golang-jwt/jwt/v5` and `prometheus/client_golang`. Do not add a GitHub SDK or an HTTP framework.
- See `README.md` for the request flow, the configuration table, and the return-code contract. See RFC 0010 for the design.

## Invariants that are easy to break
Changes in these areas need more care than the compiler or the linter can give:

- **The return-code contract.** 2xx means "do not redeliver": the event is either handled or unsatisfiable. 502 means "redeliver, this may work later". Getting this backwards is silent — a permanent failure answered 502 makes Flux retry until it gives up and drops the event, and a transient failure answered 2xx throws the deploy away immediately. `internal/githubapi.Retryable` is the single source of truth for which GitHub responses are transient; use it rather than a bare status comparison.
- **Deduplication is a claim, not a lookup.** `Tracker.Claim` reserves a key atomically; every path out of the handler must then either `Record` it (dispatch succeeded) or `Release` it (anything else). Checking `Seen` and recording after the outbound call reopens a window in which concurrent deliveries of one event all dispatch.
- **The dedup key needs a digest.** An event with no artifact digest has no identity, so it is dispatched without deduplication on purpose. Do not "fix" this by deduplicating on the empty value: every event for one product, env, reason and repo would collapse onto a single key and the first delivery would suppress the rest for the whole TTL.
- **Metric labels come from the request body.** A `CounterVec` never evicts a child, so any label taken from an Alert must be validated or bucketed first (`validate.ReasonLabel`, `validate.DispatchEvent`, `validate.RepoAllowed`). An unbounded label is a memory leak that only shows up in production.
- **`replicas: 1` is load-bearing.** Dedup state is in-memory and per-pod, so a second replica dispatches duplicates.
- **The service is unauthenticated by design.** Access control is the NetworkPolicy restricting ingress on 8080 to `flux-system`. Do not add an HTTP auth layer without changing the RFC first.

## Required verification for code changes
If you modify any Go file, you MUST run before producing a final answer/patch:

1. `make verify` — the gate the PR must pass: `gofmt`, `go vet`, and `go test -race ./...`
2. `make lint` — golangci-lint, which `make verify` does not cover

If you change `manifests/`, also run `make cdk8s-manifests-verify` — the rendered YAML in `config/` is generated and CI fails on drift.

In the final response, include the command(s) you ran and whether they passed.
If you cannot run them, you MUST say so explicitly and explain why.

## Non-negotiable
Do not claim checks passed unless you actually ran them.
