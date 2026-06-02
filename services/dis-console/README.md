# dis-console

**dis-console** is a small Go service that surfaces DIS deployment state. Its
first job is Flux: it reads the live Flux custom resources across **all
namespaces** and serves their deployment status as a read-only JSON API, so the
question *"is everything deployed and healthy, and if not, what failed and
why?"* can be answered without squinting at `flux get` or raw `kubectl ... -o
yaml`.

The name is deliberately product-level (not `flux-*`) so the Console can grow
to cover other DIS state later.

> Status: PoC. This first cut keeps the latest snapshot in memory and serves
> the API from it. Postgres persistence (status history) and the in-cluster
> deployment land in follow-up changes.

## What it reads

Azure AKS installs the upstream Flux controllers via the `microsoft.flux`
extension, so the in-cluster CRs follow the upstream schema. The Console reads
these kinds across all namespaces and resolves the served apiVersion at runtime
via a discovery `RESTMapper` (robust to Azure Flux version bumps):

| group | kind |
|---|---|
| `kustomize.toolkit.fluxcd.io` | Kustomization |
| `helm.toolkit.fluxcd.io` | HelmRelease |
| `source.toolkit.fluxcd.io` | OCIRepository, HelmRepository, HelmChart |

For each object it extracts a small normalized shape (`ready`/`reason`/
`message`/`revision`/`suspended`/`generation`/`observedGeneration`) from the
`Ready` condition and keeps the full object under `raw` for the detail endpoint.

## Endpoints

| method | path | description |
|---|---|---|
| GET | `/healthz` | liveness (always 200) |
| GET | `/readyz` | 200 only once the first sweep has completed |
| GET | `/api/summary` | counts per kind by ready state (+ suspended) |
| GET | `/api/resources?kind=&namespace=&ready=` | normalized rows; `ready=False` is the "what's broken" view |
| GET | `/api/kustomizations` | alias for `?kind=Kustomization` |
| GET | `/api/helmreleases` | alias for `?kind=HelmRelease` |
| GET | `/api/resources/{kind}/{namespace}/{name}` | single row incl. the full `raw` object |

## Run locally

Reads Flux CRs through your current kubeconfig and serves on `:8080`:

```bash
make setup-local-env   # once per fresh checkout: installs bin/golangci-lint
make run               # == go run ./cmd/main.go --local
```

```bash
curl -s localhost:8080/api/summary | jq
curl -s 'localhost:8080/api/resources?ready=False' | jq   # what's broken
curl -s localhost:8080/api/kustomizations | jq
```

Flags: `--http-address` (default `:8080`), `--poll-interval` (default `30s`),
`--local` (use kubeconfig instead of in-cluster config).

## Develop

```bash
make help                 # list targets
make run-checks-ci-cache  # fmt + vet + test + lint (the CI check suite)
make docker-build         # build the container image (podman by default)
```

See [AGENTS.md](AGENTS.md) for the required verification flow.

## In-cluster

Once deployed (follow-up change), reach it via port-forward:

```bash
kubectl -n product-dis port-forward svc/dis-console 8080:80
```
