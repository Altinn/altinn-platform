# dis-console-ui is the repository's web frontend (Vite + React + TypeScript,
# styled with Designsystemet), built and served with Bun. It is a plain Bun/npm
# package and intentionally does NOT include the shared Go ../../Makefile.common.

BUN ?= bun
# Ephemeral Bun container for host-less dev/checks (the host Bun/Node may be
# unavailable); node_modules lives in a named volume so the worktree stays clean.
BUN_IMAGE ?= oven/bun:1
NM_VOLUME ?= dis_ui_bun

APP_NAME ?= dis-console-ui
# Keep the localhost/ prefix (same convention as the dis-* operators): in local Kind
# flows we load this image directly, and because localhost/ is an explicit registry
# Kubernetes won't rewrite a bare name to docker.io/library and then fail to find the
# loaded image. Works for both podman and docker.
IMG ?= localhost/$(APP_NAME):e2e
CONTAINER_TOOL ?= podman
KIND ?= kind
KUBECTL ?= kubectl
KIND_CLUSTER ?= $(APP_NAME)-test-e2e
# Dedicated kubeconfig so kubectl/kind steps target the throwaway Kind cluster,
# never the caller's current context. Pass KIND_KUBECONFIG= (empty) to use the
# ambient kubeconfig instead (e.g. a cluster set up by helm/kind-action in CI).
KIND_KUBECONFIG ?= $(CURDIR)/.kind-$(KIND_CLUSTER).kubeconfig
E2E_PORT ?= 8080
E2E_URL ?= http://localhost:$(E2E_PORT)
# Backend for `make dev-live`: a port-forwarded dis-console server, reached from the
# dev container via podman's host alias. host.containers.internal -> your machine.
BACKEND ?= http://host.containers.internal:8080

# `make dev-cluster` port-forwards a dis-console server from a kube context (set
# KUBE_CONTEXT to e.g. your admin-test cluster) and runs the UI against it.
KUBE_CONTEXT ?=
KUBE_NS ?= product-dis
KUBE_SVC ?= dis-console
PF_PORT ?= 18080

# Drive Kind with the podman provider when the container tool is podman.
ifeq ($(CONTAINER_TOOL),podman)
export KIND_EXPERIMENTAL_PROVIDER := podman
endif

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: install
install: ## Install dependencies from the lockfile.
	$(BUN) install --frozen-lockfile

.PHONY: dev
dev: ## Start the Vite dev server (mock data by default).
	$(BUN) run dev

.PHONY: dev-podman
dev-podman: ## Start the Vite dev server in a Bun container (no host toolchain needed). Serves http://localhost:5173.
	$(CONTAINER_TOOL) run --rm -it \
		-v "$(CURDIR):/app" -v $(NM_VOLUME):/app/node_modules -w /app \
		-p 5173:5173 -e CHOKIDAR_USEPOLLING=true \
		$(BUN_IMAGE) \
		sh -lc "bun install && bun run dev -- --host 0.0.0.0"

.PHONY: dev-live
dev-live: ## Like dev-podman but against a live fleet API (BACKEND=a port-forwarded server). Proxies /api, so no CORS.
	$(CONTAINER_TOOL) run --rm -it \
		-v "$(CURDIR):/app" -v $(NM_VOLUME):/app/node_modules -w /app \
		-p 5173:5173 -e CHOKIDAR_USEPOLLING=true \
		-e VITE_USE_MOCK=false -e DIS_CONSOLE_API="$(BACKEND)" \
		$(BUN_IMAGE) \
		sh -lc "bun install && bun run dev -- --host 0.0.0.0"

.PHONY: dev-cluster
dev-cluster: ## One command: port-forward a dis-console server (set KUBE_CONTEXT=<ctx>, e.g. admin-test) and run the UI against it.
	@set -e; \
	ctxflag=""; \
	if [ -n "$(KUBE_CONTEXT)" ]; then ctxflag="--context $(KUBE_CONTEXT)"; fi; \
	echo "Port-forwarding $(KUBE_NS)/svc/$(KUBE_SVC):80 -> localhost:$(PF_PORT) (context: $(KUBE_CONTEXT))..."; \
	$(KUBECTL) $$ctxflag -n $(KUBE_NS) port-forward --address 0.0.0.0 svc/$(KUBE_SVC) $(PF_PORT):80 >/dev/null & \
	pf=$$!; \
	trap 'kill $$pf 2>/dev/null || true' EXIT; \
	printf "Waiting for the fleet API"; \
	ready=0; \
	for i in $$(seq 1 20); do \
		if ! kill -0 $$pf 2>/dev/null; then break; fi; \
		if curl -fsS -o /dev/null "http://localhost:$(PF_PORT)/api/clusters" 2>/dev/null; then ready=1; echo " — ready"; break; fi; \
		printf "."; sleep 1; \
	done; \
	if [ "$$ready" != "1" ]; then \
		echo ""; \
		echo "ERROR: no fleet API on localhost:$(PF_PORT) — the port-forward did not come up (see kubectl error above)."; \
		echo "Check: KUBE_CONTEXT='$(KUBE_CONTEXT)' is set and valid, svc/$(KUBE_SVC) exists in namespace $(KUBE_NS), and you are authenticated to that cluster."; \
		exit 1; \
	fi; \
	$(MAKE) dev-live BACKEND=http://host.containers.internal:$(PF_PORT)

.PHONY: build
build: ## Production build into dist/.
	$(BUN) run build

.PHONY: typecheck
typecheck: ## Type-check the app and the server without emitting.
	$(BUN) run typecheck

.PHONY: lint
lint: ## Run ESLint.
	$(BUN) run lint

.PHONY: test
test: ## Run unit tests.
	$(BUN) run test

.PHONY: check
check: ## Full check suite — typecheck + lint + test + build (the CI gate).
	$(BUN) run check

.PHONY: clean
clean: ## Remove build output.
	rm -rf dist

##@ Container & Kind e2e

.PHONY: image-build
image-build: ## Build the container image (Bun build + Bun-served) with $(CONTAINER_TOOL).
	$(CONTAINER_TOOL) build -t $(IMG) .

.PHONY: serve
serve: image-build ## Build + run the image on $(E2E_URL) (only needs $(CONTAINER_TOOL); no Kind, no host toolchain). Ctrl-C to stop.
	@echo "Serving $(E2E_URL) (Ctrl-C to stop)..."
	$(CONTAINER_TOOL) run --rm -it -p $(E2E_PORT):8080 $(IMG)

.PHONY: kind-create
kind-create: ## Create the Kind cluster if it does not exist (writes a dedicated kubeconfig).
	@command -v $(KIND) >/dev/null 2>&1 || { echo "kind is required: https://kind.sigs.k8s.io/docs/user/quick-start/"; exit 1; }
	@if $(KIND) get clusters 2>/dev/null | grep -qx "$(KIND_CLUSTER)"; then \
		echo "Kind cluster '$(KIND_CLUSTER)' already exists."; \
	else \
		echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
		$(KIND) create cluster --name "$(KIND_CLUSTER)"; \
	fi
	@$(KIND) export kubeconfig --name "$(KIND_CLUSTER)" --kubeconfig "$(KIND_KUBECONFIG)"

.PHONY: kind-load
kind-load: image-build ## Build and load the image into the Kind cluster.
	@tar="$$(mktemp -t dis-console-ui.XXXXXX.tar)"; \
	echo "Loading $(IMG) into Kind cluster '$(KIND_CLUSTER)'..."; \
	$(CONTAINER_TOOL) save -o "$$tar" $(IMG); \
	$(KIND) load image-archive "$$tar" --name "$(KIND_CLUSTER)"; \
	rm -f "$$tar"

.PHONY: kind-deploy
kind-deploy: ## Apply the Deployment + Service and wait for the rollout.
	KUBECONFIG="$(KIND_KUBECONFIG)" $(KUBECTL) apply -f config/kind/deployment.yaml
	@KUBECONFIG="$(KIND_KUBECONFIG)" $(KUBECTL) -n default rollout status deploy/$(APP_NAME) --timeout=120s || { \
		echo "--- rollout failed; pod state:"; \
		KUBECONFIG="$(KIND_KUBECONFIG)" $(KUBECTL) -n default get pods -l app=$(APP_NAME) -o wide; \
		KUBECONFIG="$(KIND_KUBECONFIG)" $(KUBECTL) -n default describe pod -l app=$(APP_NAME) | tail -30; \
		KUBECONFIG="$(KIND_KUBECONFIG)" $(KUBECTL) -n default logs -l app=$(APP_NAME) --tail=50 --all-containers=true || true; \
		exit 1; \
	}

.PHONY: e2e-smoke
e2e-smoke: ## Assert $(E2E_URL) serves the app's HTML + JS bundle.
	@set -e; \
	echo "Smoke-testing $(E2E_URL) ..."; \
	curl -fsS --retry 30 --retry-delay 1 --retry-connrefused -o /dev/null "$(E2E_URL)/"; \
	body="$$(curl -fsS "$(E2E_URL)/")"; \
	printf '%s' "$$body" | grep -q "<title>DIS Console</title>" || { echo "FAIL: <title>DIS Console</title> not served"; exit 1; }; \
	asset="$$(printf '%s' "$$body" | grep -oE '/assets/index-[A-Za-z0-9_-]+\.js' | head -1)"; \
	[ -n "$$asset" ] || { echo "FAIL: no /assets/index-*.js reference in index.html"; exit 1; }; \
	curl -fsS -o /dev/null "$(E2E_URL)$$asset" || { echo "FAIL: bundle $$asset not served"; exit 1; }; \
	echo "PASS: dis-console-ui serves index.html + $$asset"

.PHONY: e2e
e2e: kind-load kind-deploy ## Deploy into an existing Kind cluster and smoke-test it serves.
	@set -e; \
	pf=""; \
	trap 'if [ -n "$$pf" ]; then kill "$$pf" 2>/dev/null || true; fi' EXIT; \
	KUBECONFIG="$(KIND_KUBECONFIG)" $(KUBECTL) -n default port-forward svc/$(APP_NAME) $(E2E_PORT):80 >/dev/null 2>&1 & \
	pf=$$!; \
	$(MAKE) e2e-smoke

.PHONY: test-e2e-kind-ci
test-e2e-kind-ci: ## Full local e2e: create Kind, build+load+deploy, smoke-test, tear down.
	@set -e; \
	trap '$(KIND) delete cluster --name "$(KIND_CLUSTER)" >/dev/null 2>&1 || true; rm -f "$(KIND_KUBECONFIG)"' EXIT; \
	$(MAKE) kind-create; \
	$(MAKE) e2e

.PHONY: kind-up
kind-up: kind-create kind-load kind-deploy ## Build + deploy into Kind and leave it running for viewing.
	@echo ""
	@echo "dis-console-ui is deployed in Kind cluster '$(KIND_CLUSTER)'."
	@echo "  View it:   make kind-forward   (then open $(E2E_URL))"
	@echo "  Tear down: make kind-down"

.PHONY: kind-forward
kind-forward: ## Port-forward the in-cluster UI to $(E2E_URL) for viewing (Ctrl-C to stop).
	@echo "Forwarding $(E2E_URL) -> svc/$(APP_NAME) (Ctrl-C to stop)..."
	KUBECONFIG="$(KIND_KUBECONFIG)" $(KUBECTL) -n default port-forward svc/$(APP_NAME) $(E2E_PORT):80

.PHONY: kind-down
kind-down: ## Delete the Kind cluster and its kubeconfig.
	-$(KIND) delete cluster --name "$(KIND_CLUSTER)"
	-rm -f "$(KIND_KUBECONFIG)"
