# AGENTS.md

## What this is
dis-console-ui is the repository's **web frontend** — a React 19 + TypeScript
single-page app (built with Vite) styled with [Designsystemet](https://designsystemet.no),
built and served with **Bun**. It is a read-only console over the **dis-console
fleet API** (`services/dis-console`): releases per app and environment, what
each syncroot deploys, container images, and the DIS platform resources —
cells and chips are colored by Flux Ready state and show the deployed
revision.

It is the first JS package in this otherwise-Go monorepo and uses **Bun** as its
package manager + runtime (the production server is a small `Bun.serve` BFF in
`server/`). It does NOT use the shared Go `Makefile.common`, controller-gen,
envtest, or any of the operator toolchain.

## Data model
The UI consumes the fleet JSON API (see `services/dis-console/README.md`):
`/api/clusters`, `/api/resources` (+ `?cluster=&kind=&namespace=&ready=`),
`/api/resources/{cluster}/{kind}/{namespace}/{name}`, `/api/summary`. Flux has
no tenant id, so it is derived (with the environment) from the cluster id, which
is `<tenant>_<environment>` (e.g. `ttd_at23` → tenant `ttd`, env `at23`); the
environment is the column axis of the matrix and the tenant scopes the view. The
TypeScript types in `src/api/types.ts` mirror the Go JSON shapes exactly — keep
them in sync if the API changes.

It runs **standalone on bundled mock data by default** (`VITE_USE_MOCK=true`),
so no backend is required for development. Point it at a live `server` with
`VITE_USE_MOCK=false` and `VITE_API_BASE_URL=...`.

## Quick start
- `make install` — install dependencies from the lockfile (`bun install --frozen-lockfile`).
- `make dev` — start the Vite dev server (mock data).
- `make help` — list targets.

## Common commands
- Type-check: `make typecheck` (app `tsconfig.json` + server `tsconfig.server.json`)
- Lint: `make lint` (`eslint .`)
- Unit tests: `make test` (`vitest run`)
- Build: `make build` (`vite build` → `dist/`)

## Required verification for code changes
If you modify anything under this directory, you MUST run the full check suite
before producing a final answer/patch:

    make check

which runs `typecheck` + `lint` + `test` + `build` — the same steps CI runs
(`.github/workflows/dis-console-ui-lint-test.yml`). In the final response,
include the command(s) you ran and whether they passed. If you cannot run them,
say so explicitly and explain why.

The unit tests cover the pure data transforms only (`src/lib/*.test.ts`:
environment derivation, status mapping, and the matrix builder) — these are the
logic worth protecting; the React components are verified by the type-check and
build.

### If a local toolchain is unavailable
Run the suite in ephemeral containers instead (a named volume keeps
`node_modules` out of the worktree): install with Bun (it owns `bun.lock`),
then run the checks under **Node** —

    podman run --rm -v "$PWD:/app" -v dis_ui_bun:/app/node_modules -w /app \
      oven/bun:1 bun install --frozen-lockfile
    podman run --rm -v "$PWD:/app" -v dis_ui_bun:/app/node_modules -w /app \
      node:22 npm run check

Do NOT run `bun run check` in a node-less container (e.g. `oven/bun`): Vitest
does not support the Bun runtime, and under it the worker pool can silently
drop test files — observed as only 5 of 11 test files running, no summary
line, exit code 0. A green run that skipped tests is worse than a red one.
On hosts/CI where real `node` is on PATH, `bun run check` is fine — Bun
executes node-shebang bins with the system node.

## Container image & Kind e2e
The image (`Dockerfile`) has an `oven/bun` build stage (`bun install` + `bun run
build` → `dist/`), then a small `oven/bun` runtime stage that serves `dist/` with
the `Bun.serve` BFF (`server/index.ts`) on `:8080` — static files plus, when
`DIS_CONSOLE_API` is set, a same-origin `/api` proxy (no CORS; auth goes here
later). `config/kind/deployment.yaml` is the Deployment + Service.

The Kind e2e mirrors `services/dis-console`'s `test-e2e-kind-ci`: it stands up a
throwaway Kind cluster, builds + loads the image, deploys it, port-forwards the
Service, and asserts the app serves its HTML + JS bundle — then tears the cluster
down. It uses a **dedicated kubeconfig** (`KIND_KUBECONFIG`) so kubectl never
touches the caller's current context.

    make test-e2e-kind-ci          # full local flow (create → deploy → smoke → teardown)

To look at it running in-cluster:

    make kind-up                   # build + deploy, leaves the cluster up
    make kind-forward              # port-forward http://localhost:8080 (Ctrl-C to stop)
    make kind-down                 # delete the cluster

Requires `kind` + a container runtime (podman locally — the Makefile sets
`KIND_EXPERIMENTAL_PROVIDER=podman`; docker in CI via `CONTAINER_TOOL=docker`).
Everything (build + serve) happens inside the container, so the Kind flow works
without a host toolchain. CI runs `make e2e` against a
cluster provisioned by `helm/kind-action`
(`.github/workflows/dis-console-ui-lint-test.yml`).

## Non-negotiable
Do not claim checks passed unless you actually ran them.

## Layout
- `src/api` — the fleet API surface. `types.ts` mirrors the Go JSON; `client.ts`
  is the `FleetApi` interface; `http.ts` is the live client; `mock.ts` +
  `mock.fixtures.ts` are the bundled demo data; `index.ts` selects mock vs live
  from the **runtime config** (`/config.js`, served by the BFF from its
  environment — live when `DIS_CONSOLE_API` is set) with `VITE_USE_MOCK` as the
  dev-server fallback; `public/config.js` is the fallthrough the dev server
  serves.
- `src/lib` — pure, React-free domain logic (unit-tested): `flux.ts`
  (tenant/environment derivation, status + revision helpers, `DIS_PRODUCTS`
  kind grouping, and `isApp` — an **app** is every HelmRelease plus every
  Kustomization that is not an azapi root (roots have no kustomize labels and
  are excluded from every apps view; HelmReleases qualify unconditionally),
  `statusColor.ts`
  (the only place that maps status → Designsystemet colors), `matrix.ts`
  (`buildMatrix`: the flat resource list → apps × environments transform;
  HelmReleases and workloads fold into their owning app row via `appliedBy`
  with worst-of status per cell, while a workload applied directly by a root
  becomes its own app row — release identity = primary image tag),
  `mapLayout.ts` (the tidy left-to-right tree layout behind the syncroot Map),
  `sourceLink.ts` (repo + commit link from a Flux source's OCI annotations —
  `org.opencontainers.image.source`/`.revision`), `azure.ts` (Azure Portal
  deep-link from an ARM id), `disResources.ts` (group DIS resources by namespace
  into server→children trees), `artifacts.ts` (the base-layer artifacts ×
  environments matrix: digest extraction, class labels, rollout-in-flight
  detection, and `deployedBySyncroot` — the transitive appliedBy closure from
  a syncroot's root Kustomizations), `releases.ts` (a syncroot's releases —
  digest rows × environment stage chips, ring-ordered), `appReleases.ts` (an
  app's releases from its per-environment **status-event history** — one row
  per revision with first-seen time and current/deployed-earlier/rolling/
  failed/absent chips; plus `ownerRevisionFor` — a HelmRelease chart version
  carries no commit, so its releases link to the **owning Kustomization's**
  revision instead: the gitops commit that declared the version, matched from
  the owner's current state or time-correlated against its history),
  `workloads.ts` (an app's workloads × environments of **declared container
  images** — matched via `appliedBy`, image-ref parsing, per-container drift
  detection; feeds the syncroot **Workloads tab** and the per-release drawer,
  both empty until the fleet API sweeps workloads — schema v5).
- Page-level navigation is **hash-routed** (`src/lib/route.ts` +
  `useHashRoute`): sections and the syncroot/Kustomization pages live in the
  URL (`#/syncroots/<key>/<env>/<tab>`, `#/kustomization/<cluster>/<ns>/<name>`),
  so the browser back/forward buttons and deep links work — the syncroot page's
  active tab is a route segment too (tab switches push history, so Back walks
  Releases → Overview before leaving the page). Drawers, filters and
  sorting stay in component state deliberately — but an **open drawer traps
  Back** (`useDialogBackClose`: a same-URL sentinel history entry; Back closes
  the drawer, a UI close consumes the entry). Gotcha: Designsystemet's Dialog
  closes via the dialog *toggle* machinery, not `el.close()` — no `close`
  event fires, so `useDsDialog` (the one shared dialog-lifecycle hook) listens
  for `toggle newState === 'closed'` to sync closes back into React state.
  Page navigation renders as real anchors (`RouteLink` → `routeHash`), so
  middle-click/cmd-click/copy-link work; buttons are reserved for drawers and
  selections.
- `src/hooks` — `useFleet` (loads clusters + resources), `useResourceDetail`
  (lazily loads one resource's raw object for the drawer), `useSourceLink`
  (resolves a "view source" link, following `spec.sourceRef` to the source
  object when the resource isn't itself a source), `useArtifacts` (the
  base-layer list), `useInventory` (one Kustomization's applied-object set,
  loaded on expand), `useAppHistories` (one app's status-event history per
  environment — parallel detail fetches; an env degrades to an empty history
  on failure or an old server).
- `src/components` — Designsystemet UI: `LeftNav` (the collapsible nav
  rail — a view per DIS product; icons come from `lucide-react` (ISC), the
  only icon package to import from), `HomeView` (the landing tables),
  `DisScope`
  (the shared tenant + environment picker), `ReleasesBrowser` (the Deployments
  centerpiece — a master-detail release view: searchable app list on the left,
  the selected app's release rows on the right; also embedded in the syncroot
  page's Releases tab with the artifact as the lead entry),
  `DeploymentMatrix` (the apps × environments matrix, kept as the second
  Deployments tab), `WorkloadsTable` (workload × environment container-image
  tags, shared by the syncroot Workloads tab and `ReleaseDialog` — the drawer
  a release row opens: revision, commit link, stage chips, and the images the
  running environments declare), `StatusCell`/`StatusTag`/`StageChip` (the
  stage chips), `DetailDialog` (the
  right-side drawer), `ClustersTable`, `DisResourcesView` (one product family's
  resources — chosen by a `kinds` prop — grouped by namespace with Portal links),
  `HomeView` (the fleet landing: cluster cards + syncroot "project" cards from
  `syncrootSummaries` — namespaces/envs/worst status per syncroot),
  `SyncrootsView` (the Syncroots section: a syncroot list with per-env digest
  chips → a per-syncroot page listing everything it deployed, via the
  `deployedBySyncroot` appliedBy-closure, filterable by namespace/kind/status →
  `KustomizationPage`, one Kustomization's full inventory),
  `SortableTh` (sortable header cells backed by `lib/tableSort.ts`),
  `ArtifactDialog` (artifact drawer with origin commit link + inventory), and
  the `Alert` banners.
- `src/main.tsx` — entry point; imports the Designsystemet CSS (base + theme)
  and the Inter font. Root data-attributes live on `<html>` in `index.html`
  (`data-color-scheme="auto"` — the app follows the OS light/dark preference).
  Note: Designsystemet declares `--ds-font-family: Inter` but does NOT apply a
  font to the page — `styles.css` sets it on `body` (without that, everything
  silently renders in the browser's serif fallback).
- `src/assets` — the DIS logo (`dis-logo-color.png` for light, `dis-logo-white.png`
  for dark; the appbar swaps via a `<picture>` `prefers-color-scheme` source,
  and the color version is the favicon).
- `server/index.ts` — the production `Bun.serve` server: serves `dist/` and
  proxies `/api` to `DIS_CONSOLE_API`. Type-checked via `tsconfig.server.json`
  (Bun types), separate from the browser app's `tsconfig.json`.
