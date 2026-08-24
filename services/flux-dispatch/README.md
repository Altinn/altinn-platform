# flux-dispatch

Receives Flux reconciliation webhooks and triggers GitHub Actions workflows in
product repositories via `repository_dispatch`.

Design: [RFC 0010 — Flux reconcile webhooks](../../rfcs/0010-flux-reconcile-webhooks.md).

Product teams do not interact with this service directly. They add a Flux
`Alert` referencing the platform-provided `deploy-webhook` `Provider` and put
their dispatch target in `eventMetadata`:

```yaml
eventMetadata:
  product: "dialogporten"
  env: "at23"
  dispatch_repo: "Altinn/dialogporten"   # required, must be in the Altinn org
  dispatch_event: "flux-deploy"          # optional, defaults to flux-deploy
```

## Request flow

`POST /flux-events` runs the RFC 0010 flow in order:

1. Read the body behind a 64 KB `http.MaxBytesReader` — oversized bodies get
   `413` with no parsing attempted.
2. Parse the Flux event — `200` if the body is not a Flux event; retrying
   cannot help, so Flux is not told to resend it.
3. Ignore unrecognised reconciliation reasons.
4. Require `dispatch_repo`, and require it to match
   `^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$` and start with `Altinn/`.
5. Route by reason: a `dispatch_event` ending in `-failed` receives failure
   reasons only, any other value receives success reasons only. Flux's
   `eventSeverity: info` forwards errors too, so this filter is what keeps a
   failure from arriving as a success event.
6. Skip events already dispatched for the same
   `{product}/{env}/{reason}/{digest}/{dispatch_repo}`.
7. Authenticate as the GitHub App (cached installation token) and
   `POST /repos/{dispatch_repo}/dispatches`.

The endpoint itself is not authenticated at the HTTP layer. Access control is
enforced by the `flux-dispatch-allow-webhook-traffic` NetworkPolicy, which
only permits ingress to port 8080 from the `flux-system` namespace — the only
namespace that ever calls this service (notification-controller). See RFC
0010 for the full rationale.

### Return codes

| Code | When |
|---|---|
| `200` | Dispatched, deduplicated, ignored, unparseable, or rejected for a config reason — retrying cannot help |
| `413` | Body larger than 64 KB |
| `502` | Transient GitHub failure (5xx, timeout, auth outage) — Flux retries with backoff |

`413` is the only non-2xx answer to a delivered request. An unparseable body
is acknowledged with `200` and a warning log carrying
`outcome=invalid_payload`: Flux collapses every non-2xx into a single "failed
to send notification" class, so a `4xx` would be indistinguishable from a real
outage on its side while the log already carries the full diagnostic.

`dispatch_repo` is matched **case-sensitively** against the `Altinn/` prefix, so
a lowercase `altinn/...` is rejected even though GitHub treats owner names
case-insensitively. Segments consisting only of dots (`Altinn/..`, `Altinn/.`)
are rejected too — `url.JoinPath` cleans them rather than failing, so they would
otherwise rewrite the outbound request path.

## Accepted reconciliation reasons

kustomize-controller v1 event reasons (`fluxcd/pkg/apis/meta`):

- Success: `ReconciliationSucceeded`
- Failure: `ReconciliationFailed`, `BuildFailed`, `HealthCheckFailed`,
  `PruneFailed`, `DependencyNotReady`, `ArtifactFailed`

`HealthCheckFailed` is only emitted for Kustomizations configured with
`wait`/`healthChecks`. Anything else is acknowledged with `200` and ignored.

## Configuration

| Variable | Required | Default | Description |
|---|---|---|---|
| `DRY_RUN` | no | `false` | Log-only mode: run the full request flow but skip the outbound GitHub call |
| `GITHUB_APP_ID` | yes, unless `DRY_RUN=true` | | GitHub App ID |
| `GITHUB_INSTALLATION_ID` | yes, unless `DRY_RUN=true` | | App installation ID |
| `GITHUB_PRIVATE_KEY_PATH` | yes, unless `DRY_RUN=true` | | Path to the PEM private key; must exist and be readable at startup unless `DRY_RUN=true` |
| `GITHUB_API_URL` | no | `https://api.github.com` | GitHub API base |
| `DEDUP_TTL` | no | `24h` | How long a dispatched event is remembered |
| `DEDUP_MAX_ENTRIES` | no | `10000` | Dedup tracker cap; oldest is evicted |
| `LISTEN_ADDR` | no | `:8080` | Webhook listener |
| `METRICS_ADDR` | no | `:9090` | Prometheus listener |
| `DEFAULT_DISPATCH_EVENT` | no | `flux-deploy` | Used when the Alert omits `dispatch_event` |

Set `DRY_RUN=true` to deploy and validate the Flux → service half of this
service end-to-end before a GitHub App exists: every step of the request flow
still runs, but the outbound `repository_dispatch` call is replaced with a log
line, so no GitHub App, private key, or Key Vault secret is needed for the
first rollout.

## Endpoints

| Port | Path | Purpose |
|---|---|---|
| 8080 | `POST /flux-events` | Webhook receiver |
| 8080 | `GET /healthz` | Liveness |
| 8080 | `GET /readyz` | Readiness |
| 9090 | `GET /metrics` | Prometheus exposition |

## Metrics

`flux_dispatch_events_received_total{reason}`,
`flux_dispatch_dispatches_total{repo,event_type,reason}`,
`flux_dispatch_dispatch_errors_total{repo,event_type,error_code}`,
`flux_dispatch_dedup_hits_total{reason}`, `flux_dispatch_dedup_entries`,
`flux_dispatch_github_auth_errors_total`,
`flux_dispatch_dispatch_duration_seconds{repo}`,
`flux_dispatch_dryrun_dispatches_total{repo,event_type,reason}`.

`flux_dispatch_dryrun_dispatches_total` only increments while `DRY_RUN=true`;
`flux_dispatch_dispatches_total` does not move in that mode, so a dashboard
built on it never reports a dispatch that did not happen.

They live on a dedicated registry, so the metrics port exposes only these.

## Development

```sh
make verify   # gofmt -l . && go vet ./... && go test -race ./...
make test     # race tests with coverage
make build    # bin/flux-dispatch
```

Dedup state is in-memory: a pod restart costs at most one extra dispatch per
environment, which is harmless for idempotent workflows.
