- Feature Name: flux_reconcile_webhooks
- Start Date: 2026-03-06
- RFC PR: [altinn/altinn-platform#0000](https://github.com/altinn/altinn-platform/pull/0000)
- Github Issue: [altinn/altinn-platform#0000](https://github.com/altinn/altinn-platform/issues/0000)
- Product/Category: CI/CD
- State: **REVIEW**

# Summary
[summary]: #summary

A shared platform service (`flux-dispatch`) that receives Flux reconciliation webhooks and triggers GitHub Actions workflows in product repositories via `repository_dispatch`. The service handles both successful deployments (e.g., trigger e2e tests) and reconciliation failures (e.g., trigger incident workflows or rollback automation). Product teams configure **what** to trigger (target repo and event type) through Flux Alert metadata — the platform team owns and operates the webhook infrastructure. No product team needs to manage webhook URLs, GitHub App credentials, or deduplication logic.

# Motivation
[motivation]: #motivation

Products deploying through the pull-based CD system (RFC 0001) currently have no automated way to know when their application has been successfully deployed to an environment. This means:

- **E2e tests are triggered manually** or on a timer, not when the deployment actually completes. This leads to either delayed feedback or wasted CI minutes testing against stale deployments.
- **No deployment-event-driven automation.** Teams cannot wire post-deploy steps (smoke tests, notifications, DORA metric collection) to the actual moment Flux finishes reconciling.
- **No automated response to deployment failures.** When Flux fails to reconcile, teams find out through manual monitoring or delayed alerts. There is no way to automatically trigger incident workflows, rollback automation, or failure notifications through GitHub Actions.
- **Each product would need to build their own solution.** Without a shared service, every team wanting deploy-triggered workflows would need to solve GitHub App authentication, webhook deduplication, and Flux payload parsing independently.

The expected outcome is that any product team can configure "on deploy (or deploy failure), run my GitHub Actions workflow" by adding a Flux Alert manifest and a workflow file — nothing else.

# Guide-level explanation
[guide-level-explanation]: #guide-level-explanation

## Concepts

### Flux Alert
A Flux custom resource that watches a specific Kubernetes resource (e.g., a Kustomization) and fires a webhook when events occur (reconciliation succeeded, failed, etc.). Alerts live in the product's namespace alongside their other Flux resources.

### Flux Provider
A Flux custom resource that defines _where_ to send Alert notifications. The platform provides a base Provider manifest pointing to the `flux-dispatch` service — product teams include this in their syncroot without modification.

### Repository Dispatch
A GitHub API mechanism (`POST /repos/{owner}/{repo}/dispatches`) that triggers workflows in a repository with a custom event type and payload. This is how external systems trigger GitHub Actions.

### Dispatch Target
The GitHub repository and event type a product wants triggered on deployment. Products specify this through `eventMetadata` on their Flux Alert — they never interact with webhook URLs or the dispatch service directly.

## How it works

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────────────────────────┐
│ Flux         │     │ flux-dispatch    │     │ GitHub                          │
│ notification │────>│ (platform svc)   │────>│                                 │
│ controller   │     │                  │     │  Altinn/dialogporten            │
└─────────────┘     │  - HMAC verify   │     │    └── .github/workflows/       │
                    │  - GitHub App    │     │        └── e2e-on-deploy.yml    │
                    │    auth          │     │                                 │
                    │  - dispatch      │────>│  Altinn/correspondence          │
                    │                  │     │    └── .github/workflows/       │
                    └──────────────────┘     │        └── e2e-on-deploy.yml    │
                                            └─────────────────────────────────┘
```

1. Product CI pushes a new app OCI artifact (already happens today)
2. Flux detects the new artifact and reconciles the app Kustomization
3. Flux notification-controller fires an HMAC-signed webhook to the `flux-dispatch` service
4. The service verifies the HMAC signature, validates `dispatch_repo` format and org prefix
5. The service deduplicates (skips routine reconciles where nothing changed)
6. The service reads `dispatch_repo` and `dispatch_event` from the event metadata
7. The service authenticates as a GitHub App and sends `repository_dispatch` to the target repo, including the reconciliation `reason` in the payload
8. The product's GitHub Actions workflow runs (e.g., e2e tests on success, incident response on failure)

## What product teams do

Product teams configure **two things** — no webhook URLs, no credentials, no service changes:

### 1. Add a Flux Alert to their syncroot

The platform provides a base Provider manifest. Products include it and add an Alert specifying their dispatch target:

**Success alert** — triggers on successful reconciliation (e.g., run e2e tests):

> **Note on `eventSeverity`:** Flux's `eventSeverity: info` forwards **all** events (including errors) — it is not a success-only filter. The `flux-dispatch` service filters events server-side by the `reason` field: only reasons indicating success (e.g., `ReconciliationSucceeded`) are dispatched for Alerts using `dispatch_event: "flux-deploy"`. Products wanting only failure events should use `eventSeverity: error`, which Flux filters at the Alert level.

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Alert
metadata:
  name: deploy-webhook
  namespace: product-{name}
spec:
  providerRef:
    name: deploy-webhook           # references the platform-provided Provider
  eventSeverity: info              # forwards all events; service filters by reason
  eventSources:
    - kind: Kustomization
      name: {product}-apps-{env}   # the app Kustomization (NOT the syncroot)
  eventMetadata:
    product: "{name}"
    env: "{environment}"
    dispatch_repo: "Altinn/{repo}" # which GitHub repo to trigger
    dispatch_event: "flux-deploy"  # optional, defaults to "flux-deploy"
```

**Failure alert** — triggers on reconciliation failure (e.g., incident workflow or rollback):

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Alert
metadata:
  name: deploy-failure-webhook
  namespace: product-{name}
spec:
  providerRef:
    name: deploy-webhook
  eventSeverity: error              # only fires on errors
  eventSources:
    - kind: Kustomization
      name: {product}-apps-{env}
  eventMetadata:
    product: "{name}"
    env: "{environment}"
    dispatch_repo: "Altinn/{repo}"
    dispatch_event: "flux-deploy-failed"  # distinct event type for failures
```

Products can use one or both Alerts. Using separate `dispatch_event` values lets the same repo have different workflows for success vs. failure.

The `eventMetadata` fields are what the product team controls:
- `dispatch_repo` — the GitHub repo where the workflow lives (e.g., `Altinn/dialogporten`)
- `dispatch_event` — the event type to trigger (defaults to `flux-deploy`)
- `product` and `env` — passed through to the workflow as context

### 2. Add a GitHub Actions workflow to their repo

**Success workflow** (e.g., run e2e tests after deploy):

```yaml
name: E2E Tests on Deploy
on:
  repository_dispatch:
    types: [flux-deploy]

jobs:
  e2e-tests:
    name: "E2E - ${{ github.event.client_payload.environment }}"
    runs-on: ubuntu-latest
    environment: ${{ github.event.client_payload.environment }}
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.event.client_payload.commit_sha }}

      - name: Run e2e tests
        run: |
          echo "Product:     ${{ github.event.client_payload.product }}"
          echo "Environment: ${{ github.event.client_payload.environment }}"
          echo "Commit SHA:  ${{ github.event.client_payload.commit_sha }}"
          # run actual e2e tests here
```

**Failure workflow** (e.g., incident response or rollback):

```yaml
name: Deploy Failure Response
on:
  repository_dispatch:
    types: [flux-deploy-failed]

jobs:
  on-failure:
    name: "Deploy failed - ${{ github.event.client_payload.environment }}"
    runs-on: ubuntu-latest
    steps:
      - name: Notify and respond
        run: |
          echo "FAILURE in ${{ github.event.client_payload.environment }}"
          echo "Reason:  ${{ github.event.client_payload.reason }}"
          echo "Message: ${{ github.event.client_payload.message }}"
          # trigger incident workflow, notify Slack, initiate rollback, etc.
```

> **Important:** `repository_dispatch` only triggers workflows on the repository's **default branch** (usually `main`). Workflow files must be merged to the default branch before they can receive dispatches. This means product teams cannot test their dispatch workflows on a feature branch using real events — use `workflow_dispatch` with manual test payloads for pre-merge testing.

That's it. No platform team involvement to onboard, no PRs to altinn-platform.

## What the platform team manages

- The `flux-dispatch` Go service (deployment, monitoring, upgrades)
- The GitHub App registration and installation (controls which repos can be dispatched to)
- The base Provider manifest that products include in their syncroot
- The service endpoint URL (cluster-internal, products never see it)

## Workflow payload

The `repository_dispatch` event payload available to workflows:

**Success event:**

```json
{
  "event_type": "flux-deploy",
  "client_payload": {
    "product": "dialogporten",
    "environment": "at23",
    "commit_sha": "abc1234def5678",
    "revision": "at23@sha256:aabbccdd",
    "kustomization_name": "dialogporten-apps-at23",
    "reason": "ReconciliationSucceeded",
    "message": "Applied revision at23@sha256:aabbccdd"
  }
}
```

**Failure event:**

```json
{
  "event_type": "flux-deploy-failed",
  "client_payload": {
    "product": "dialogporten",
    "environment": "at23",
    "commit_sha": "abc1234def5678",
    "revision": "at23@sha256:aabbccdd",
    "kustomization_name": "dialogporten-apps-at23",
    "reason": "ReconciliationFailed",
    "message": "kustomize build failed: ... (truncated)"
  }
}
```

| Field | Description |
|---|---|
| `product` | Product name from Alert `eventMetadata` |
| `environment` | Environment name (at23, tt02, yt01, prod) |
| `commit_sha` | Source commit SHA extracted from Flux `originRevision` |
| `revision` | Full OCI revision digest — useful for audit/debugging |
| `kustomization_name` | The Flux Kustomization that reconciled |
| `reason` | Flux reconciliation reason (e.g., `ReconciliationSucceeded`, `ReconciliationFailed`) |
| `message` | Human-readable message from Flux describing the event (truncated to 1024 chars) |

# Reference-level explanation
[reference-level-explanation]: #reference-level-explanation

## Service architecture

**Language:** Go (matches existing platform services like `lakmus`).

**Location:** `services/flux-dispatch/` in the `altinn-platform` repo.

**Dependencies (minimal):**
- `golang-jwt/jwt/v5` for GitHub App JWT generation (RS256)
- `prometheus/client_golang` for metrics exposition
- stdlib for everything else: `net/http`, `crypto/hmac`, `log/slog`, `sync`, `encoding/json`
- No GitHub SDK — `repository_dispatch` is a single POST

## Request flow

```
POST /flux-events (from Flux notification-controller)
  │
  ├── 1. Verify HMAC-SHA256 signature from X-Signature header against shared secret
  │      └── Invalid/missing signature → 401 Unauthorized (logged, not retried)
  ├── 2. Parse JSON body into FluxEvent struct (request body limited to 64 KB via http.MaxBytesReader)
  ├── 3. Validate: reason is a recognized reconciliation event and required metadata present
  │      ├── Accepted reasons: "ReconciliationSucceeded", "ReconciliationFailed",
  │      │   "ValidationFailed", "DependencyNotReady", "ArtifactFailed"
  │      └── Unrecognized reason → 200 OK (non-retryable, Flux should not retry)
  ├── 4. Reject if dispatch_repo missing → 200 OK + log warning
  ├── 5. Validate dispatch_repo format and org prefix
  │      ├── Must match `^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$` (strict owner/repo format)
  │      ├── Must start with `Altinn/` (reject cross-org dispatch attempts)
  │      └── Invalid → 200 OK + log warning (non-retryable config issue)
  ├── 6. Construct GitHub API URL using url.JoinPath (prevents path traversal)
  ├── 7. Dedup check: has this (product, env, reason, OCI-digest, dispatch_repo) been seen before?
  │      └── Yes → 200 OK + log "skipping duplicate event"
  ├── 8. Extract commit SHA from originRevision ("main/abc123" → "abc123")
  ├── 9. Authenticate as GitHub App (cached installation token)
  ├── 10. POST /repos/{dispatch_repo}/dispatches with client_payload (includes reason + message)
  │       ├── Success → record in dedup tracker, increment metrics → 200 OK
  │       └── Failure → log error, increment error metric → 502 (Flux retries on 5xx)
  └── Done
```

**Return code strategy:** Always 2xx for validation/config errors (retrying won't help). Only 5xx for transient failures (GitHub API down) so Flux retries.

**Event routing:** Flux's `eventSeverity: error` filters at the Alert level (only error events are sent). However, `eventSeverity: info` forwards **all** events, including errors. To avoid dispatching failure events through a success Alert, the service filters by the `reason` field: for Alerts using `dispatch_event: "flux-deploy"` (or the default), only success reasons (`ReconciliationSucceeded`) are dispatched. For Alerts using a failure-specific `dispatch_event` (e.g., `flux-deploy-failed`), only failure reasons are dispatched. This ensures products get the correct event type regardless of the Alert's `eventSeverity` setting.

## Deduplication

Flux reconciles on an interval (e.g., every 10 minutes) even when nothing changed, emitting `ReconciliationSucceeded` each time. Without dedup, every product would get a `repository_dispatch` every 10 minutes.

**Design:** In-memory map keyed by `{product}/{env}/{reason}/{sha256-digest}/{dispatch_repo}`. Including `reason` in the key ensures that a success and a failure for the same digest are treated as distinct events (both get dispatched). Including `dispatch_repo` ensures that a single Kustomization dispatching to multiple repos (e.g., one for e2e tests and one for a dashboard) does not incorrectly deduplicate the second dispatch. Background goroutine evicts entries older than a configurable TTL (default 24h).

**Capacity limit:** The dedup map is capped at a configurable maximum number of entries (default 10,000). When the cap is reached, the oldest entry is evicted. This prevents unbounded memory growth from crafted or high-volume events. The `flux_dispatch_dedup_entries` gauge metric tracks the current map size for monitoring.

**Pod restart:** Dedup state is lost — at worst one extra dispatch per environment. Acceptable because workflows are idempotent (running e2e tests twice is harmless).

## GitHub App authentication

1. Load PEM private key from file (mounted from Kubernetes Secret / Azure Key Vault)
2. Generate JWT (RS256, `iss` = App ID, 10min expiry)
3. Exchange JWT for installation access token (`POST /app/installations/{id}/access_tokens`)
4. Cache token until 5min before expiry (tokens valid for 1 hour)

**Permissions required:** Contents: Read & write (for `repository_dispatch`).

**Security boundary:** The GitHub App is installed on specific repos. A product setting `dispatch_repo: Altinn/some-other-repo` will get a 404 from GitHub if the app isn't installed there.

## Platform-provided base Provider

The platform provides a Provider manifest that all products include via their syncroot's kustomization:

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Provider
metadata:
  name: deploy-webhook
spec:
  type: generic-hmac
  address: http://flux-dispatch.dis-platform.svc.cluster.local:8080/flux-events
  secretRef:
    name: flux-dispatch-hmac-token
---
apiVersion: v1
kind: Secret
metadata:
  name: flux-dispatch-hmac-token
type: Opaque
stringData:
  token: "${HMAC_TOKEN}"  # provisioned by platform; shared between Provider and service
```

The `generic-hmac` provider type causes Flux to sign each webhook payload with an HMAC-SHA256 signature sent in the `X-Signature` header. The `flux-dispatch` service verifies this signature before processing, ensuring only authentic Flux notifications are accepted.

Products include this in their syncroot base and reference it from their Alerts with `providerRef.name: deploy-webhook`. The namespace is set by the product's Kustomization `targetNamespace`.

## Prometheus metrics

The service exposes metrics on `GET /metrics` (port 9090) using the standard Prometheus client library. Flux's built-in metrics cover controller reconciliation health and HTTP transport, but have no visibility into outbound webhook delivery success/failure, deduplication, or per-product dispatch activity. Including metrics directly in the service is the standard Go pattern and avoids the overhead of a separate metrics sidecar.

| Metric | Type | Labels | Description |
|---|---|---|---|
| `flux_dispatch_events_received_total` | Counter | `reason` | Total webhook events received from Flux, by reconciliation reason |
| `flux_dispatch_dispatches_total` | Counter | `repo`, `event_type`, `reason` | Successful `repository_dispatch` calls to GitHub |
| `flux_dispatch_dispatch_errors_total` | Counter | `repo`, `event_type`, `error_code` | Failed `repository_dispatch` calls (label: HTTP status or timeout) |
| `flux_dispatch_dedup_hits_total` | Counter | `reason` | Events skipped by deduplication |
| `flux_dispatch_dedup_entries` | Gauge | | Current number of entries in the dedup tracker |
| `flux_dispatch_github_auth_errors_total` | Counter | | Failures obtaining GitHub App installation token |
| `flux_dispatch_dispatch_duration_seconds` | Histogram | `repo` | Latency of outbound `repository_dispatch` API calls |

**Why not a separate metrics service or Flux built-in metrics?**

- Flux's notification-controller exposes HTTP-level metrics (`gotk_event_*`) for its own event server, but has no counters for outbound webhook delivery, per-alert dispatch counts, or deduplication. These gaps mean we cannot rely on Flux alone for observability.
- A separate sidecar or metrics aggregation service would add deployment complexity for little benefit. The metrics are intrinsic to the dispatch logic — the service already knows when it dispatches, deduplicates, or errors. Exposing them directly is simpler and more reliable.
- The `/metrics` endpoint runs on a separate port (9090) from the webhook handler (8080) so it can be scraped by Prometheus without exposing it to Flux's notification-controller.

## HTTP server hardening

The Go HTTP server is configured with explicit timeouts and limits to prevent resource exhaustion:

```go
server := &http.Server{
    Addr:              ":8080",
    ReadTimeout:       10 * time.Second,
    ReadHeaderTimeout: 5 * time.Second,
    WriteTimeout:      30 * time.Second,
    MaxHeaderBytes:    1 << 16, // 64 KB
}
```

Request bodies are limited to 64 KB via `http.MaxBytesReader` before JSON parsing (see request flow step 2). This prevents oversized payloads from consuming memory.

## Kubernetes deployment

- Single-replica Deployment in `dis-platform` namespace
- ClusterIP Service: `flux-dispatch.dis-platform.svc.cluster.local:8080`
- Metrics port: `9090` (scraped by Prometheus via `PodMonitor` or `ServiceMonitor`)
- Health endpoints: `GET /healthz` (liveness), `GET /readyz` (readiness)
- GitHub App private key from Kubernetes Secret (sourced from Azure Key Vault)
- HMAC shared secret from Kubernetes Secret (sourced from Azure Key Vault, shared with Flux Provider)
- No external ingress — cluster-internal only

### NetworkPolicy

Following existing patterns (`dis-pgsql-operator`, `dis-apim-operator`, `dis-identity-operator`), the service defines NetworkPolicies to restrict network access:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: flux-dispatch-allow-webhook-traffic
  namespace: dis-platform
spec:
  podSelector:
    matchLabels:
      app: flux-dispatch
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: flux-system
      ports:
        - protocol: TCP
          port: 8080
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: flux-dispatch-allow-metrics-traffic
  namespace: dis-platform
spec:
  podSelector:
    matchLabels:
      app: flux-dispatch
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: monitoring
      ports:
        - protocol: TCP
          port: 9090
```

This ensures only the Flux notification-controller can reach the webhook handler (port 8080) and only Prometheus can scrape metrics (port 9090).

## Interaction with existing features

- **RFC 0001 (pull-based CD):** The `originRevision` metadata already set by product CI provides the commit SHA — no pipeline changes needed.
- **Flux notification-controller:** Deployed with `--watch-all-namespaces=true`, so Alerts in product namespaces work out of the box.
- **Multi-tenancy:** Azure `multiTenancy.enforce=true` only blocks cross-namespace refs. Same-namespace Alert → Kustomization works fine.

## Corner cases

| Scenario | Behavior |
|---|---|
| Product omits `dispatch_repo` in eventMetadata | 200 OK, log warning, no dispatch |
| `dispatch_repo` has invalid format or non-Altinn org | 200 OK, log warning, no dispatch (non-retryable config issue) |
| GitHub App not installed on target repo | GitHub returns 404, service logs error, returns 200 (non-retryable config issue) |
| Invalid or missing HMAC signature | 401 Unauthorized, log warning, no dispatch |
| Request body exceeds 64 KB | 413 Request Entity Too Large, no parsing attempted |
| Rapid consecutive deploys (different digests) | Each has unique OCI digest, dedup correctly allows each through |
| Same Kustomization dispatches to two different repos | Both dispatched — dedup key includes `dispatch_repo` |
| Reconciliation failure event | Dispatched with `reason: "ReconciliationFailed"` and the error `message` from Flux |
| Same digest fails then succeeds | Both dispatched — dedup key includes `reason`, so they are distinct events |
| Repeated failures with same digest | Deduplicated — only the first failure for a given digest triggers dispatch |
| `eventSeverity: info` Alert receives error event | Service filters by `reason` field — error reasons are not dispatched through success-type Alerts |
| Dedup map reaches capacity (10,000 entries) | Oldest entry evicted, new event processed normally |
| Service pod restarts | Dedup state lost, at most one duplicate dispatch per env (harmless) |
| GitHub API downtime | Service returns 502, Flux retries with backoff |

# Drawbacks
[drawbacks]: #drawbacks

- **New service to maintain.** Another pod in the cluster, another GitHub App to manage. However, the service is small (~500 LoC Go), stateless, and follows existing patterns (`lakmus`).
- **Trusts product-provided `dispatch_repo`.** A product could target another product's repo within the `Altinn/` org. Mitigated by strict format validation (owner/repo regex), `Altinn/` org prefix enforcement, and GitHub App installation scope (can only dispatch to installed repos). A per-product allowlist can be added later if needed.
- **Single point of failure.** If the service is down, no dispatches fire. Flux will retry on 5xx, so brief outages self-heal. For extended outages, dispatches queue in Flux and fire when the service recovers.

# Rationale and alternatives
[rationale-and-alternatives]: #rationale-and-alternatives

## Why `repository_dispatch` over `workflow_dispatch`?

`repository_dispatch` is designed for external event triggers with freeform `client_payload`. `workflow_dispatch` requires predefined inputs and targets a specific workflow file. Since each product may structure their workflows differently, `repository_dispatch` is more flexible — products define their own workflows and parse the payload as needed.

## Why a shared platform service instead of per-product solutions?

GitHub App authentication, Flux payload parsing, deduplication, and `repository_dispatch` logic is identical for every product. Centralizing it means products only deal with YAML config and workflow files — no code, no credentials.

## Why self-service routing via `eventMetadata`?

A config file in the service would require a PR + redeploy for every new product. By reading `dispatch_repo` from the Flux Alert's `eventMetadata`, products onboard without any service changes. The GitHub App installation scope provides the trust boundary.

## Why not use Flux's built-in GitHub provider?

Flux has a `github` provider type, but it creates commit statuses / deployment statuses — not `repository_dispatch` events. We need to trigger arbitrary workflows, which requires `repository_dispatch`.

## What is the impact of not doing this?

Teams would either run e2e tests on a timer (wasteful, delayed feedback), build their own webhook receivers (duplicated effort), or skip post-deploy testing entirely.

# Prior art
[prior-art]: #prior-art

- **Flux notification-controller** supports webhooks to generic endpoints. This RFC leverages that capability rather than building custom Flux controllers.
- **GitHub `repository_dispatch`** is the standard mechanism for external-to-GitHub event triggering. Used by Netlify, Vercel, and CircleCI for "deploy completed → trigger action" patterns.
- **ArgoCD Notifications** provides similar functionality (triggers on sync completion) with built-in GitHub integration. Our approach achieves the same using Flux's native notification system.
- **RFC 0001 (pull-based CD)** anticipated this need: "Provide useful Observability signals that can be used further for debugging or higher degree metrics, e.g DORA metrics."

# Unresolved questions
[unresolved-questions]: #unresolved-questions

- ~~Should we maintain an allowlist of permitted `dispatch_repo` values?~~ **Resolved:** The service enforces an `Altinn/` org prefix and strict format validation. A per-product allowlist is deferred to "Future possibilities" until there is evidence of cross-product dispatch being a concern.
- What naming convention for the GitHub App? (e.g., `dis-flux-dispatch`)
- Should the platform-provided Provider manifest live in a shared OCI artifact or be documented for products to copy?
- What Kubernetes namespace label should be used for the Flux notification-controller namespace in the NetworkPolicy? (e.g., `kubernetes.io/metadata.name: flux-system`)

# Future possibilities
[future-possibilities]: #future-possibilities

- **DORA metrics.** The service observes every successful deployment with commit SHA and timestamp. This data could feed deployment frequency and lead time calculations.
- **GitHub Deployment Statuses.** The service could create GitHub Deployment + Deployment Status objects, giving teams a deployment history view in the GitHub UI.
- **Multi-event support.** Products could configure multiple dispatch targets from a single Alert (e.g., trigger e2e tests AND update a deployment dashboard).
- **Cross-cluster aggregation.** If products deploy to multiple clusters, the service could aggregate events and only trigger workflows when all clusters for an environment are reconciled.
- **Per-product `dispatch_repo` allowlist.** The current design validates the `Altinn/` org prefix. A more granular allowlist mapping product namespaces to specific permitted repos could be added if cross-product dispatch becomes a concern.
