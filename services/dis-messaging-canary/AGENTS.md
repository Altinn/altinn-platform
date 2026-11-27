# AGENTS.md

## What this is
dis-messaging-canary is a small Go service that tests dis-messaging
([RFC 0018](../../rfcs/0018-self-service-messaging.md)) from the position of a
product app. On a timer it sends one message to an Azure Service Bus queue,
receives it and completes it. It exposes Prometheus metrics about these round
trips.

It is NOT a kubebuilder operator: no controllers and no CRDs. It includes the
shared `../../Makefile.common` and overrides the build and test wiring.

## Common commands
- Install the tools once per fresh worktree: `make setup-local-env`
- Format: `make fmt-cache`
- Vet: `make vet-cache`
- Lint: `make lint-cache`
- Unit tests: `make test-cache`

The `*-cache` targets use a `GOCACHE` inside the repo. They are for local use.
CI runs the bare targets.

## Required verification for code changes
If you change files under `cmd/**` or `internal/**`, you MUST run this before
you give a final answer or patch:

    make run-checks-ci-cache

It runs fmt, vet, test and lint. In the final response, give the commands that
you ran and their result. If you cannot run them, say so and say why.

## Non-negotiable
Do not claim checks passed unless you actually ran them.

## Layout
- `internal/probe` — the round-trip loop behind the `Messenger` interface, and
  the metrics. The unit tests use a fake `Messenger`, so they need no Azure.
  All metric names are in `metrics.go`. Dashboards, alert rules and SLOs read
  them: do not rename a metric without a plan for its readers.
