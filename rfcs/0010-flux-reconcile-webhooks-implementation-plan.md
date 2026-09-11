- Document: Implementation Plan
- Companion RFC: [RFC 0010 — Flux Reconcile Webhooks](0010-flux-reconcile-webhooks.md)
- RFC PR: [altinn/altinn-platform#3220](https://github.com/Altinn/altinn-platform/pull/3220)
- Start Date: 2026-08-07
- Product/Category: CI/CD
- Status: Living checklist, updated as tasks land — not state-tracked like an RFC (no REVIEW/ACCEPTED/REJECTED)

# Flux Deploy Webhooks (`flux-dispatch`) — Implementation Plan

**Goal:** Ship the `flux-dispatch` platform service and supporting manifests/docs so any product team can run a GitHub Actions workflow when Flux finishes (or fails) reconciling their app — configured purely via a Flux Alert + a workflow file.

**Architecture:** Flux notification-controller (per-product Alert + platform-provided Provider) → `flux-dispatch` Go service in the `dis-platform` namespace (validates, deduplicates on OCI digest, authenticates as a GitHub App) → `repository_dispatch` to the product repo → the product's workflow runs. [RFC 0010](0010-flux-reconcile-webhooks.md) is the normative spec for service behavior; this document tracks how it gets built, by whom, and in which repo.

**Tech Stack:** Go (stdlib + `golang-jwt/jwt/v5` + `prometheus/client_golang`), Flux `notification.toolkit.fluxcd.io/v1beta3`, Kustomize overlays, GitHub App + `repository_dispatch`, cdk8s manifests + GitOps deploy via the `gitops-manifests` OCI pattern (mirroring the existing `lakmus` service).

## Rollout order

1. **Service** — build `flux-dispatch` in `altinn-platform` (Tasks 4–6): the Go service, CI, and Kubernetes manifests. Task 3 (GitHub App) is not needed yet — see step 3 below.
2. **Deploy to the test cluster with `DRY_RUN=true`, at23 first** — ship via `gitops-manifests` (Task 7); the ring mechanics promote outward from `at_ring1` once verified. This validates the Flux → service half end-to-end before any GitHub App exists.
3. **Dry-run validation, then enable dispatching** — confirm events arrive from dialogporten's Alert, validation and `-failed` routing behave, and dedup behaves across reconcile intervals, all by reading logs and `flux_dispatch_dryrun_dispatches_total` (Task 10 Step 0). Then complete Task 3 (GitHub App registration + secrets) and flip `DRY_RUN=false`.
4. **Pilot dialogporten** — finish the dialogporten-manifests wiring and run the end-to-end pilot at at23 with dispatching enabled (Tasks 8, 10), including the failure path.
5. **Docs** — rewrite the core-repo product guide once the design is proven in the pilot (Task 9).
6. **Remaining envs** — promote dialogporten's Alerts (and onboard further products) to tt02, yt01, and prod once the at23 pilot is verified end to end.

## Repos

| Repo | Role in this plan |
|---|---|
| `Altinn/altinn-platform` | RFC 0010, this plan (Tasks 1–2); the `flux-dispatch` service, CI, and Kubernetes manifests (Tasks 4–6) |
| `dis-way/gitops-manifests` | GitOps deployment package that ships `flux-dispatch` to clusters (Task 7) |
| `Altinn/dialogporten-manifests` | Pilot product wiring — Provider, Alert, and namespace fixes for dialogporten (Task 8) |
| `dis-way/core` | Product-facing operator guide, replacing the pre-RFC design doc (Task 9) |

Task 3 (GitHub App registration + Key Vault secrets) is GitHub-org/Azure administration, not scoped to a single repo. Task 10 (end-to-end pilot verification) exercises `Altinn/dialogporten` and `Altinn/dialogporten-manifests` together against the at23 cluster.

## Context

Products on the pull-based CD system (RFC 0001) have no automated "deploy finished" signal — e2e tests run on timers, and reconciliation failures trigger nothing. The design landed on these **locked decisions** (do not re-litigate):

1. **Watch the app `Kustomization`** (not the syncroot, not the OCIRepository) — gives both success and failure events, fires on app deploys only.
2. **Deduplicate on the OCI digest** in `kustomize.toolkit.fluxcd.io/revision` at the receiver. Dedup is needed regardless of cadence (controller restarts, spec-change re-applies, and notification redeliveries all re-emit for an unchanged digest) — but whether current Flux re-emits `ReconciliationSucceeded` on every no-op interval reconcile is unverified; measure it during the pilot (Task 10 Step 0) before treating dedup-hit metrics as meaningful.
3. **Platform-owned receiver**: product teams define *which GitHub Actions get triggered*, **not** webhook URLs. The platform team runs the webhook receiver and fires the repository dispatches (explicit user requirement).
4. **`repository_dispatch` over `workflow_dispatch`** — freeform `client_payload`, products control routing via `on.repository_dispatch.types`. Rationale recorded in RFC 0010 §"Rationale and alternatives".
5. Receiver = **GitHub App** (`flux-dispatch` Go service), `Altinn/` org enforcement, in-memory dedup (TTL 24h, cap 10 000), Prometheus metrics, NetworkPolicies restricting the webhook endpoint to `flux-system` (the endpoint is unauthenticated at the HTTP layer — access control is the NetworkPolicy, not a shared secret; see RFC 0010 §NetworkPolicy). All specified in RFC 0010.

### Current gaps this plan closes

- The `flux-dispatch` service does not exist yet (Tasks 4–7).
- dialogporten-manifests draft PR #30 predates the RFC 0010 design: the Provider is `type: generic` with a placeholder address, the Alert has no `dispatch_repo`/`dispatch_event`, and there is a namespace inconsistency between the Alert/Provider and the app Kustomization (Task 8).
- The core-repo product guide contradicts the final design — it still describes product-managed webhook URLs and receivers (Task 9).

## Global Constraints

- RFC 0010 is the behavioral spec — request flow, corner-case table, metrics table, NetworkPolicies are normative. When the plan says "per RFC §X", read that section before implementing.
- Conventional Commits in every repo (`feat:`, `fix:`, `chore:`, `rfc:`, `docs:`).
- Go service: stdlib-first; only `golang-jwt/jwt/v5` and `prometheus/client_golang` as deps. No GitHub SDK.
- HTTP return-code strategy: **2xx for validation/config errors** (Flux must not retry), **5xx only for transient failures** (GitHub API down). 413 for oversized body.
- `dispatch_repo` must match `^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$` **and** start with `Altinn/`.
- Ports: webhook `:8080`, metrics `:9090`. Namespace: `dis-platform`. Service DNS: `flux-dispatch.dis-platform.svc.cluster.local`.
- Accepted Flux reasons: `ReconciliationSucceeded` (success); `ReconciliationFailed`, `BuildFailed`, `HealthCheckFailed`, `PruneFailed`, `DependencyNotReady`, `ArtifactFailed` (failure). Anything else → 200 OK, ignored. (`ValidationFailed` is a pre-v1 kustomize-controller reason that no longer exists; `BuildFailed` — bad manifest — is the most common real failure, and `PruneFailed` is reachable because `prune: true` is set. Re-verify the set against kustomize-controller v1 source in Task 4d.)
- Event routing convention (closes an RFC ambiguity — recorded here in the Task 1 doc): a `dispatch_event` ending in `-failed` receives **failure** reasons only; any other value (including default `flux-deploy`) receives **success** reasons only.
- Dedup key: `{product}/{env}/{reason}/{sha256-digest}/{dispatch_repo}`; record **after** successful dispatch; TTL 24h; cap 10 000 entries (evict oldest).

---

### Task 1: Implementation-plan document (`Altinn/altinn-platform`) — **DONE**

This document. Committed alongside RFC 0010 so reviewers of [PR #3220](https://github.com/Altinn/altinn-platform/pull/3220) can see how the RFC will actually be built.

### Task 2: Open the RFC PR (`Altinn/altinn-platform`) — **DONE**

- [x] **Step 1:** PR is open: [Altinn/altinn-platform#3220](https://github.com/Altinn/altinn-platform/pull/3220) — "RFC 0010: Add failure dispatch and metrics to flux-dispatch", base `main`, head `arealmaas/flux-dispatch-rfc`, not a draft.
- [x] **Step 2:** Header placeholders replaced (commit `rfc: link RFC 0010 PR`): RFC PR now links `#3220`, and `Github Issue: N/A` — `rfcs/README.md` §"What the process is" is still `TBD`, so no issue is mandated; RFC 0011 sets the `N/A` precedent.
- [ ] **Step 3:** Do **not** merge — RFC state is `REVIEW`; humans review. Implementation (Tasks 3+) can proceed in parallel on separate branches, marked as depending on RFC acceptance.

### Task 3: GitHub App registration + secrets (GitHub org + Azure Key Vault — no single repo)

This task is mostly clicking in GitHub/Azure; the deliverable is a checklist the platform team executes. **Not a blocker for the first deploy:** `DRY_RUN=true` (see Rollout order) lets `flux-dispatch` ship and be validated end-to-end without a GitHub App; this task is required before the *dispatching* phase — i.e. before flipping `DRY_RUN=false` — not before deployment. Record outcomes (App ID, installation ID) below once executed.

- [ ] **Step 1:** Create GitHub App in the `Altinn` org: name **`dis-flux-dispatch`** (confirmed 2026-08-07). Permissions: **Contents: Read & write** (required for `repository_dispatch`). Webhook: disabled. Where can it be installed: Only this org.
- [ ] **Step 2:** Generate a private key (PEM). Note the **App ID**.
- [ ] **Step 3:** Install the App on the pilot repo `Altinn/dialogporten` (installation scope = the security boundary; add repos as products onboard). Note the **Installation ID**.
- [ ] **Step 4:** Store the private key (from Step 2) in Azure Key Vault (platform vault, same pattern as other `dis-*` service secrets): secret `flux-dispatch-github-app-key` (PEM).

**Outcomes** (fill in once Steps 1–3 are executed; do not invent values):

| Field | Value |
|---|---|
| GitHub App name | `dis-flux-dispatch` (confirmed 2026-08-07) |
| App ID | _pending — record after Step 2_ |
| Installation ID | _pending — record after Step 3_ |

### Task 4: `flux-dispatch` Go service (`Altinn/altinn-platform`)

**Files (mirror `services/lakmus` layout):**
```
services/flux-dispatch/
  go.mod                          # module github.com/altinn/altinn-platform/services/flux-dispatch
  cmd/main.go
  internal/config/config.go       + config_test.go
  internal/event/event.go         + event_test.go
  internal/validate/validate.go   + validate_test.go
  internal/dedup/tracker.go       + tracker_test.go
  internal/githubauth/app.go      + app_test.go
  internal/dispatch/dispatcher.go + dispatcher_test.go
  internal/metrics/metrics.go
  internal/server/server.go       + server_test.go   # integration
  Makefile  README.md
```

Work TDD per package: write the test, see it fail, implement, see it pass, commit (`feat(flux-dispatch): ...`). Suggested branch: `arealmaas/flux-dispatch-service` off `main`.

**Sub-task 4a — config** (`internal/config`): `Load() (Config, error)` from env vars: `GITHUB_APP_ID`, `GITHUB_INSTALLATION_ID`, `GITHUB_PRIVATE_KEY_PATH`, `GITHUB_API_URL` (default `https://api.github.com`), `DEDUP_TTL` (default `24h`), `DEDUP_MAX_ENTRIES` (default `10000`), `LISTEN_ADDR` (`:8080`), `METRICS_ADDR` (`:9090`), `DEFAULT_DISPATCH_EVENT` (`flux-deploy`).
- [ ] Tests: missing required var → error naming the var; defaults applied; invalid `DEDUP_TTL` → error. Implement, pass, commit.

**Sub-task 4b — event types + parsing** (`internal/event`):
```go
type FluxEvent struct {
    InvolvedObject struct {
        Kind, Name, Namespace string
    } `json:"involvedObject"`
    Severity  string            `json:"severity"`
    Reason    string            `json:"reason"`
    Message   string            `json:"message"`
    Timestamp time.Time         `json:"timestamp"`
    Metadata  map[string]string `json:"metadata"`
}
func Parse(r io.Reader) (FluxEvent, error)                 // json.Decode + basic shape check
func (e FluxEvent) CommitSHA() string                      // metadata["kustomize.toolkit.fluxcd.io/originRevision"]: "main/abc123" → "abc123" (last "/" segment — branch names may contain "/"); "" if absent
func (e FluxEvent) Revision() string                       // metadata["kustomize.toolkit.fluxcd.io/revision"], e.g. "at23@sha256:aabbccdd"
func (e FluxEvent) Digest() string                         // "at23@sha256:aabbccdd" → "sha256:aabbccdd"; bare "sha256:…" passed through; "" if absent — this is the dedup-key digest
func (e FluxEvent) Meta(key string) string                 // product, env, dispatch_repo, dispatch_event lookups
```
- [ ] Tests: parse the sample payload from RFC §"Webhook payload" (copy it verbatim into the test); `CommitSHA()` on `"main/abc1234def5678"` → `"abc1234def5678"`, on `"feature/x/abc123"` → `"abc123"`, on missing key → `""`; `Digest()` on `"at23@sha256:aabbccdd"` → `"sha256:aabbccdd"`, on bare `"sha256:aabbccdd"` → unchanged, on missing → `""`; unknown JSON fields ignored. Implement, pass, commit.

**Sub-task 4d — validation** (`internal/validate`):
```go
var successReasons = map[string]bool{"ReconciliationSucceeded": true}
var failureReasons = map[string]bool{ // kustomize-controller v1 event reasons — re-verify against source when implementing
    "ReconciliationFailed": true, "BuildFailed": true, "HealthCheckFailed": true,
    "PruneFailed": true, "DependencyNotReady": true, "ArtifactFailed": true,
}
var repoRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)
func KnownReason(reason string) bool
func RepoAllowed(repo string) error        // regex + strings.HasPrefix(repo, "Altinn/")
func ShouldDispatch(reason, dispatchEvent string) bool
// success reason  → dispatchEvent does NOT end in "-failed"
// failure reason  → dispatchEvent ends in "-failed"
```
- [ ] Tests (from RFC corner-case table): `Altinn/dialogporten` ok; `Evil/repo` rejected; `Altinn/a/b`, `Altinn/../x`, empty → rejected; `ShouldDispatch("ReconciliationSucceeded","flux-deploy")` true; `("ReconciliationFailed","flux-deploy")` false; `("ReconciliationFailed","flux-deploy-failed")` true; `("BuildFailed","flux-deploy-failed")` true; `("HealthCheckFailed","flux-deploy")` false; `("ReconciliationSucceeded","flux-deploy-failed")` false; unknown reason false; table-driven check that every reason in both maps satisfies `KnownReason`. Implement, pass, commit.

**Sub-task 4e — dedup tracker** (`internal/dedup`):
```go
func New(ttl time.Duration, maxEntries int, gauge prometheus.Gauge) *Tracker
func (t *Tracker) Key(product, env, reason, digest, repo string) string  // "/" joined; digest = event.Digest() ("sha256:…"), not the full revision
func (t *Tracker) Seen(key string) bool          // read-only check
func (t *Tracker) Record(key string)             // insert + evict-oldest at cap + set gauge
func (t *Tracker) StartEviction(ctx context.Context, every time.Duration) // background TTL sweep
```
- [ ] Tests: `Seen` false → `Record` → `Seen` true; different reason or repo in key → not seen; at `maxEntries=2`, third `Record` evicts the oldest; entries older than TTL swept (inject a `now func() time.Time` for testability — no sleeping); concurrent `Seen`/`Record` race-free (`go test -race`). Implement (mutex + map[string]time.Time + insertion-order list), pass, commit.

**Sub-task 4f — GitHub App auth** (`internal/githubauth`):
```go
func New(appID, installationID string, pem []byte, apiBase string, hc *http.Client) *App
func (a *App) Token(ctx context.Context) (string, error)
```
JWT: RS256, claims `iss`=appID, `iat`=now-60s, `exp`=now+10m (golang-jwt/v5). Exchange: `POST {apiBase}/app/installations/{id}/access_tokens` with `Authorization: Bearer <jwt>` → cache returned token; refresh when < 5 min to expiry.
- [ ] Tests with `httptest.Server` as fake GitHub: first `Token()` hits the endpoint (assert Bearer JWT parses with the test public key), second call within validity is served from cache (endpoint hit count stays 1); expiring token triggers refresh; non-201 response → error. Generate a throwaway RSA key in the test. Implement, pass, commit.

**Sub-task 4g — dispatcher** (`internal/dispatch`):
```go
type Payload struct {
    Product           string `json:"product"`
    Environment       string `json:"environment"`
    CommitSHA         string `json:"commit_sha"`
    Revision          string `json:"revision"`
    KustomizationName string `json:"kustomization_name"`
    Reason            string `json:"reason"`
    Message           string `json:"message"` // truncate to 1024 chars before send
}
func (d *Dispatcher) Send(ctx context.Context, repo, eventType string, p Payload) error
```
`POST {apiBase}/repos/{repo}/dispatches` (build with `url.JoinPath`) body `{"event_type": eventType, "client_payload": {...}}`, headers `Authorization: Bearer <installation token>`, `Accept: application/vnd.github+json`. GitHub 204 → nil; 4xx → `ErrNonRetryable` (wrapped, caller returns 200); 5xx/timeout → `ErrRetryable` (caller returns 502).
- [ ] Tests with `httptest.Server`: asserts URL path `/repos/Altinn/dialogporten/dispatches`, event_type and payload fields round-trip, message >1024 chars truncated; 404 → ErrNonRetryable; 500 → ErrRetryable. Implement, pass, commit.

**Sub-task 4h — metrics** (`internal/metrics`): define exactly the seven collectors from RFC §"Prometheus metrics" (`flux_dispatch_events_received_total{reason}`, `flux_dispatch_dispatches_total{repo,event_type,reason}`, `flux_dispatch_dispatch_errors_total{repo,event_type,error_code}`, `flux_dispatch_dedup_hits_total{reason}`, `flux_dispatch_dedup_entries` gauge, `flux_dispatch_github_auth_errors_total`, `flux_dispatch_dispatch_duration_seconds{repo}` histogram) on a dedicated registry. No tests beyond compile; asserted via the integration test.

**Sub-task 4i — server wiring** (`internal/server` + `cmd/main.go`): implement RFC §"Request flow" steps 1–9 in order as one handler on `POST /flux-events`; `http.MaxBytesReader` 64 KB **before** reading (413 on overflow); server timeouts exactly per RFC §"HTTP server hardening"; `GET /healthz` + `GET /readyz` on 8080; metrics registry served on 9090; `log/slog` JSON logs with `product`, `env`, `repo`, `reason`, `outcome` fields.
- [ ] Integration test (`server_test.go`): boot the full handler with fake GitHub backend; table-driven over the RFC corner-case table — happy path success event → 200 + one dispatch; duplicate digest → 200 + no second dispatch (`dedup_hits` +1); missing `dispatch_repo` → 200 + no dispatch; non-Altinn repo → 200 + no dispatch; >64 KB body → 413; GitHub 500 → 502; failure reason with `dispatch_event: flux-deploy-failed` → dispatched; same digest fail-then-succeed → both dispatched. Implement, pass, commit.
- [ ] **Final step:** `cd services/flux-dispatch && go vet ./... && go test -race ./...` all green; `gofmt -l .` clean. Commit any wiring leftovers.

### Task 5: Dockerfile, Makefile, CI (`Altinn/altinn-platform`)

**Files:** `services/flux-dispatch/Dockerfile`, `services/flux-dispatch/Makefile`, `.github/workflows/flux-dispatch-lint-test.yml`, `.github/workflows/flux-dispatch-release.yml`

- [ ] **Step 1:** Copy `services/lakmus/Dockerfile` and `Makefile` as templates; adjust binary name/paths (multi-stage build, distroless/static final stage — match lakmus exactly).
- [ ] **Step 2:** Create both workflows by copying `lakmus-lint-test.yml` / `lakmus-release.yml` and swapping paths/names (`services/flux-dispatch/**` path filters, image name `flux-dispatch`). Keep the same registry/tagging/release-please conventions the lakmus workflows use — read them first, don't guess.
- [ ] **Step 3 — release-please registration (without this nothing ever publishes):** altinn-platform releases through the repo-central release-please setup, not per-service workflows alone. Add `services/flux-dispatch` to `release-please-config.json` (mirror the lakmus entry: `{"release-type": "simple", "component": "flux-dispatch"}`) and seed `.release-please-manifest.json` with an initial version for `services/flux-dispatch`. Only then does a release PR open and the release workflow publish the image/OCI artifact that Tasks 6–7 deploy.
- [ ] **Step 4:** Run the same lint/test commands the workflow runs locally; commit `ci(flux-dispatch): add lint-test and release workflows`.

### Task 6: Kubernetes manifests (`Altinn/altinn-platform`, cdk8s like lakmus)

**Files:** `services/flux-dispatch/manifests/` — bootstrap by copying `services/lakmus/manifests/` and rewriting the synth entrypoint. **Note:** lakmus's manifests were migrated from TypeScript cdk8s to Go (`manifests/main.go` + `internal/k8scompat/`) on `main` — read the current lakmus layout before copying, the plan's original `main.ts`/`package.json` assumption is stale.

- [ ] **Step 1:** Define: single-replica Deployment (`replicas: 1` with a comment that in-memory dedup forbids horizontal scaling; image `flux-dispatch`, ports 8080/9090, liveness `/healthz`, readiness `/readyz`, env from Task 4a, volume mounting Secret `flux-dispatch-github-app-key`; resources small, e.g. 50m/64Mi requests); ClusterIP Service `flux-dispatch` port 8080; the three NetworkPolicies **verbatim from RFC §NetworkPolicy** (ingress 8080 from `flux-system`, ingress 9090 from `monitoring`, plus the egress policy for GitHub API 443 + DNS — required on any cluster with a default-deny egress baseline; check the other `dis-*` operators' policies for the local egress convention). The namespace-label question is closed: `kubernetes.io/metadata.name` is auto-set on every namespace (k8s ≥ 1.21) — only confirm the literal names (`flux-system` is the Azure Flux extension default; `monitoring` exists, lakmus targets it); PodMonitor/ServiceMonitor for 9090 (whichever lakmus uses); an ExternalSecret for the Key Vault secret (same mechanism the other `dis-*` services use — check their manifests first).
- [ ] **Step 2:** Synth renders without error; `kubectl apply --dry-run=client -f dist/` passes. Commit `feat(flux-dispatch): kubernetes manifests`.

### Task 7: Deploy via gitops-manifests (`dis-way/gitops-manifests`)

**Files:** create `oci/flux-dispatch/` mirroring `oci/lakmus/` — which has **four** manifest files (`namespace.yaml`, `oci-repository.yaml`, `flux-kustomize.yaml`, `kustomization.yaml`) plus `CHANGELOG.md`/`README.md` (release-please owns the changelog). Swap name/namespace (`dis-platform`) and artifact ref to the OCI artifact `flux-dispatch-release.yml` publishes (match lakmus's artifact naming exactly).

- [ ] **Step 1:** Create the package. **Deliberate deviation from lakmus:** lakmus's `namespace.yaml` is commented out because `monitoring` pre-exists — `dis-platform` does **not** exist, so the Namespace must actually be included (uncommented and listed in `kustomization.yaml` resources).
- [ ] **Step 2 — release/ring wiring (without this the package never deploys):** register the component the same way `oci-lakmus` is registered:
  - `release-please-config.json`: add an `oci/flux-dispatch` package entry (`{"release-type": "simple", "component": "oci-flux-dispatch"}`) and seed the release-please manifest.
  - `oci/releaseconfig.json`: add per-ring version keys for the new component (`at_ring1` … `prod_ring2`), initialized to the first release version.
  - Copy lakmus's renovate version-comment convention in `oci-repository.yaml`/`flux-kustomize.yaml` so ring bumps automate.
- [ ] **Step 3:** PR to gitops-manifests (base per that repo's convention). Rollout follows the ring mechanics: the first release lands in `at_ring1` and promotes outward per `releaseconfig.json` — verify on the at rings before promoting further. Commit `release: deploy flux-dispatch to dis-platform`.

### Task 8: Finish dialogporten-manifests draft PR #30 (`Altinn/dialogporten-manifests`)

Continue on draft PR **#30** in `Altinn/dialogporten-manifests`, pushing to that PR's head branch. It already carries an initial Provider/Alert, but one that predates the RFC 0010 design (see gaps above).

- [ ] **Step 1 — fix the Alert target (blocking):** the real object is Kustomization **`dialogporten-apps`** in ns **`product-dialogporten`** (`flux/syncroot/base/dialogporten-flux-kustomization.yaml`) — same name in every env; the overlays patch only `spec.path` (verified in-repo, no cluster access needed). The committed Alert (ns `dialogporten`, watching `dialogporten-apps-{env}`) is wrong on both name and namespace, and this is worse than a matching bug: the syncroot's fluxConfiguration is namespace-scoped (`scope = "namespace"`, namespace `product-dialogporten` — `dis_products_syncroot_multitenancy` in dis-modules), so a syncroot object declaring any other namespace **fails RBAC and breaks the whole syncroot reconciliation**. Alert, Provider, and Secret/ExternalSecret must all live in `product-dialogporten`, and `eventSources[0].name` must be `dialogporten-apps` (Flux `eventSources` are same-namespace under multi-tenancy). Optional sanity check when cluster access is handy: `kubectl get kustomizations -A | grep dialogporten`.
- [ ] **Step 2:** Update `flux/syncroot/base/deploy-webhook-provider.yaml` to the RFC design:
```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Provider
metadata:
  name: deploy-webhook
  namespace: product-dialogporten
spec:
  type: generic
  address: http://flux-dispatch.dis-platform.svc.cluster.local:8080/flux-events
```
- [ ] **Step 3:** Update `flux/syncroot/base/deploy-webhook-alert.yaml`: namespace `product-dialogporten`, `eventSources[0].name: dialogporten-apps`, and `eventMetadata` gains `dispatch_repo: "Altinn/dialogporten"` + `dispatch_event: "flux-deploy"` (keep `product`/`env`). Then shrink the env overlay patches (`flux/syncroot/{at23,tt02,yt01,prod}/kustomization.yaml`): today they also pin `eventSources[0].name: dialogporten-apps-{env}` — that must go. Since name/namespace/metadata are now identical everywhere, each overlay patch needs only the per-env `env` value (kustomize strategic-merge *merges* maps — patches carry just the keys that differ). **Note:** the `at22` overlay was removed on `main`, so only these four overlays remain.
- [ ] **Step 4 — failure Alert (pilot scope):** add `deploy-failure-webhook` (copy RFC §"Failure alert": `eventSeverity: error`, `dispatch_event: "flux-deploy-failed"`) in the **at23 overlay only** for now — Task 10 Step 3 exercises it; promote to base once proven.
- [x] **Step 5 — success semantics (decided + applied 2026-08-07):** `wait: true` + `timeout: 5m` added to `flux/syncroot/base/dialogporten-flux-kustomization.yaml` (commit `d5ad530`, pushed to both branch refs). `ReconciliationSucceeded` now fires when workloads are healthy (kstatus), not merely applied, and `HealthCheckFailed` is reachable for the failure Alert. Tune the timeout if normal rollouts ever approach 5m.
- [ ] **Step 6:** Verify every overlay renders: `for e in at23 tt02 yt01 prod; do kustomize build flux/syncroot/$e >/dev/null || echo "FAIL $e"; done`. Commit `feat: point deploy webhook at flux-dispatch`, push to the PR head branch, then `gh pr ready 30` once flux-dispatch is deployed (leave draft until then).

### Task 9: Rewrite the core-repo product guide (`dis-way/core`)

Work in `dis-way/core`, on the existing feature branch that carries the flux-reconcile-webhooks doc changes — the guide rewrite continues there rather than starting fresh. PR against that repo's `main`.

- [ ] **Step 1:** Rewrite `FLUX-RECONCILE-WEBHOOKS.md` for the final design. Keep: overview of the two-level OCI architecture, "watch the app Kustomization", payload field docs. Replace the DIY sections (product-managed Provider `address`, product HMAC secret, receiver/dedup/version-mapping guidance) with: include the platform-provided Provider, add the Alert with `dispatch_repo`/`dispatch_event` `eventMetadata` (copy the two Alert examples from RFC §"What product teams do"), add a `repository_dispatch` workflow (copy the success + failure workflow examples), document the `client_payload` fields table, note the default-branch limitation of `repository_dispatch`, link to RFC 0010.
- [ ] **Step 2:** `git add FLUX-RECONCILE-WEBHOOKS.md README.md && git commit -m "docs: add flux deploy webhook product guide"`; push and open a PR to `main` (include affected path per repo convention).

### Task 10: End-to-end pilot verification (dialogporten @ at23) (`Altinn/dialogporten-manifests` + `Altinn/dialogporten`)

**Precondition:** the dry-run phase (Rollout order step 3) already deployed `flux-dispatch` with `DRY_RUN=true` and confirmed events arrive, validation/`-failed` routing behaves, and dedup behaves across reconcile intervals — without GitHub in the loop. This task re-runs the same checks with `DRY_RUN=false` and a live GitHub App, so it assumes dispatching already works end-to-end at the transport level rather than starting from a standing start.

- [ ] **Step 0 — verify the dedup premise:** with the success Alert live, watch two-plus no-op reconcile intervals (`flux events --for Kustomization/dialogporten-apps -n product-dialogporten`, or the service's `events_received_total`) and record whether Flux actually re-emits `ReconciliationSucceeded` when nothing changed (notification-controller also rate-limits identical payloads, ~5m window). This validates locked decision 2's cadence assumption; if no-op intervals do not re-emit, dedup still guards restarts/re-applies/redeliveries, but the dedup-hit assertions below become best-effort.
- [ ] **Step 1:** Merge a trivial workflow into `Altinn/dialogporten` default branch: `on: repository_dispatch: types: [flux-deploy]`, one job echoing `github.event.client_payload` (RFC has the exact YAML — §"Add a GitHub Actions workflow"). (`repository_dispatch` only triggers workflows on the default branch.)
- [ ] **Step 2:** With flux-dispatch deployed and PR #30 merged to at23: push a new app OCI artifact → confirm the workflow ran with correct `commit_sha`/`environment`; wait ≥ 2 reconcile intervals → confirm **no** second run (the hard assertion); read `flux_dispatch_dedup_hits_total` as informational — it only climbs if Flux re-emitted (Step 0); push a syncroot-only change → no run.
- [ ] **Step 3 — failure path (required; it is the RFC's headline feature):** merge a `repository_dispatch: types: [flux-deploy-failed]` workflow (RFC failure example), temporarily break a manifest in at23 → confirm the at23 failure Alert (Task 8 Step 4) dispatches with the real reason (`BuildFailed` or `ReconciliationFailed`), the failure workflow runs, and the success Alert stays silent; revert the break and confirm the recovery dispatches success.
- [ ] **Step 4:** Mark PR #30 ready for review; update RFC 0010 `State` when the process calls for it.

## Open decisions (proposed defaults — confirm with team during review, none block Tasks 1–2)

| Question (from RFC §Unresolved) | Proposed default |
|---|---|
| GitHub App name | `dis-flux-dispatch` — **confirmed** |
| How products get the base Provider manifest | Copy-paste snippet in the product guide now; shared OCI artifact later |
| HMAC token delivery to product namespaces | **Resolved — HMAC dropped 2026-08-12.** The endpoint is unauthenticated at the HTTP layer; access control is the NetworkPolicy restricting inbound traffic to `flux-system`. |
| Failure-event convention | `dispatch_event` suffix `-failed` = failure route (encoded in `validate.ShouldDispatch`) |
| `wait: true` on app Kustomizations | **Decided + applied** (Task 8 Step 5, dialogporten-manifests `d5ad530`): success means *healthy*, and `HealthCheckFailed` is reachable |
| NetworkPolicy namespace labels | Resolved — `kubernetes.io/metadata.name` is auto-set on every namespace (k8s ≥ 1.21); only the literal names needed confirming (`flux-system` = Azure Flux extension default, `monitoring` exists) |
| RFC number | Resolved — `main` has 0009 and 0011–0013 but no 0010; the 0010 slot is held by open PR #3220, so numbering is consistent once that merges |

## Verification (per repo, minimum bar)

- **altinn-platform:** `cd services/flux-dispatch && gofmt -l . && go vet ./... && go test -race ./...`; manifests synth + `kubectl apply --dry-run=client`; CI workflows green on the PR; `services/flux-dispatch` present in both `release-please-config.json` and `.release-please-manifest.json`.
- **gitops-manifests:** `oci/flux-dispatch` registered in `release-please-config.json` and `oci/releaseconfig.json` (all rings); the package's kustomize build renders the Namespace.
- **dialogporten-manifests:** `kustomize build` all four overlays; `gh pr checks 30`.
- **core:** docs only — README link resolves; guide examples match RFC verbatim.
- **End-to-end:** Task 10 is the real verification — a deploy to at23 triggers exactly one workflow run, interval reconciles trigger zero.
