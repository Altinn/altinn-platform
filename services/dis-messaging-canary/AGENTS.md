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
- Build the binary: `make build-cache`
- Build the image: `make docker-build` (podman, image `localhost/dis-messaging-canary:local`)

The `*-cache` targets use a `GOCACHE` inside the repo. They are for local use.
CI runs the bare targets.

## Image and release
- `.github/workflows/dis-messaging-canary-release.yml` builds the image and
  scans it with Trivy on each pull request that changes this folder. A finding
  of severity HIGH or CRITICAL that has a fix fails the run.
- Each merge to `main` that changes this folder pushes
  `ghcr.io/altinn/altinn-platform/dis-messaging-canary:latest`.
- release-please makes the release pull request, the tag
  `dis-messaging-canary-v<version>` and the changelog. The release pushes the
  image with the tag `v<version>`.
- A manifest gets the image through the pull-through registry of the clusters:
  `altinncr.azurecr.io/ghcr.io/altinn/altinn-platform/dis-messaging-canary:v<version>`.
- The image has no shell and its user is `65532`. The canary writes no files:
  run it with `runAsNonRoot`, a read-only root file system and no added
  capabilities.

## Required verification for code changes
If you change files under `cmd/**` or `internal/**`, you MUST run this before
you give a final answer or patch:

    make run-checks-ci-cache

It runs fmt, vet, test and lint. In the final response, give the commands that
you ran and their result. If you cannot run them, say so and say why.

## Non-negotiable
Do not claim checks passed unless you actually ran them.

## Layout
- `cmd/main.go` — reads the settings, starts the loop, and serves `/metrics`
  and `/healthz`.
- `internal/config` — the settings from the environment: `ENDPOINT`, `QUEUE`,
  `CANARY_INTERVAL`, `CANARY_TIMEOUT`, `CANARY_METRICS_ADDR`.
- `internal/probe` — the round-trip loop behind the `Messenger` interface, and
  the metrics. The unit tests use a fake `Messenger`, so they need no Azure.
  All metric names are in `metrics.go`. Dashboards, alert rules and SLOs read
  them: do not rename a metric without a plan for its readers.
- `internal/servicebus` — the `Messenger` for one queue in an Azure Service Bus
  namespace. `cmd/main.go` gives it the workload identity of the pod.
