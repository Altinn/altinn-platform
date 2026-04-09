# Image URL to use for build/deploy targets.
# Keep the localhost/ prefix: in local Kind flows we load this image directly,
# and this avoids accidental registry lookups when the image name is overridden.
IMG ?= localhost/controller:latest
KIND_IMAGE_ARCHIVE := bin/dis-vault-operator_image.tar

# Local Go cache settings for CI check runs
CACHE_DIR ?= $(CURDIR)/.cache
GOCACHE ?= $(CACHE_DIR)/go-build
WORKSPACE_ROOT ?= $(abspath $(CURDIR)/../..)
WORKSPACE_GO_VERSION ?= $(shell awk '/^go /{print $$2; exit}' "$(WORKSPACE_ROOT)/services/dis-identity-operator/go.mod" 2>/dev/null)
CERT_MANAGER_INSTALL_SKIP ?= true

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= podman

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: workspace
workspace: ## Create local go.work for dev across dis-vault-operator and dis-identity-operator (not committed).
	@cd "$(WORKSPACE_ROOT)" && \
	if [ -f go.work ]; then \
		echo "go.work already exists; updating modules"; \
		go work use ./services/dis-vault-operator ./services/dis-identity-operator; \
	else \
		go work init ./services/dis-vault-operator ./services/dis-identity-operator; \
	fi
	@if [ -n "$(WORKSPACE_GO_VERSION)" ]; then \
		cd "$(WORKSPACE_ROOT)" && go work edit -go=$(WORKSPACE_GO_VERSION); \
	fi
	@echo "Local workspace at $(WORKSPACE_ROOT)/go.work (ignored by git)."

.PHONY: workspace-clean
workspace-clean: ## Remove local go.work/go.work.sum (dev only).
	@rm -f "$(WORKSPACE_ROOT)/go.work" "$(WORKSPACE_ROOT)/go.work.sum"

.PHONY: setup-local-env
setup-local-env: ## Bootstrap local dev environment (shared by Codex/Claude/Gemini).
	$(MAKE) workspace
	$(MAKE) cache-setup
	$(MAKE) kustomize
	$(MAKE) controller-gen
	$(MAKE) golangci-lint
	$(MAKE) setup-envtest
	@mkdir -p "$(CACHE_DIR)/golangci-lint"
	@echo "Local environment bootstrap completed."

.PHONY: clean
clean: workspace-clean tidy verify-deps lint test ## Clean local dev artifacts before pushing.

.PHONY: tidy
tidy: ## Run go mod tidy.
	go mod tidy

.PHONY: verify-deps
verify-deps: ## Verify go.mod/go.sum resolve from a clean module cache.
	@tmp_modcache="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp_modcache"' EXIT INT TERM; \
	echo "Verifying module pins with clean module cache: $$tmp_modcache"; \
	GOWORK=off GOFLAGS=-mod=readonly GOMODCACHE="$$tmp_modcache" go mod download

##@ Cache

CACHE_TARGETS := run-checks-ci

.PHONY: cache-setup
cache-setup:
	@mkdir -p "$(GOCACHE)"

$(CACHE_TARGETS): export GOCACHE := $(GOCACHE)
$(CACHE_TARGETS): cache-setup

%-cache: export GOCACHE := $(GOCACHE)
%-cache: cache-setup
	$(MAKE) $*

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	"$(CONTROLLER_GEN)" rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	"$(CONTROLLER_GEN)" object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: test-ci
test-ci: manifests generate fmt vet setup-envtest ## Run tests (CI style).
	DISVAULT_SKIP_ENVTEST=1 KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: run-checks-ci
run-checks-ci: ## Run CI checks with local Go caches.
	$(MAKE) fmt
	$(MAKE) generate
	$(MAKE) manifests
	$(MAKE) test-ci

# TODO(user): To use a different vendor for e2e tests, modify the setup under 'tests/e2e'.
# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# CertManager is installed by default; skip with:
# - CERT_MANAGER_INSTALL_SKIP=true
KIND_CLUSTER ?= dis-vault-operator-test-e2e
KIND_KUBECONFIG ?= $(LOCALBIN)/kubeconfig-$(KIND_CLUSTER)

.PHONY: setup-test-e2e
setup-test-e2e: ## Set up a Kind cluster for e2e tests if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac
	@mkdir -p "$(LOCALBIN)"
	@$(KIND) get kubeconfig --name $(KIND_CLUSTER) > "$(KIND_KUBECONFIG)"

.PHONY: test-e2e
test-e2e: setup-test-e2e manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	@test_status=0; cleanup_status=0; \
	KUBECONFIG=$(KIND_KUBECONFIG) CERT_MANAGER_INSTALL_SKIP=$(CERT_MANAGER_INSTALL_SKIP) KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -ginkgo.v || test_status=$$?; \
	$(MAKE) cleanup-test-e2e || cleanup_status=$$?; \
	if [ $$test_status -ne 0 ]; then exit $$test_status; fi; \
	exit $$cleanup_status

.PHONY: install-aso-crds-kind
install-aso-crds-kind: setup-test-e2e aso-crds ## Install ASO CRDs and disable conversion webhooks for Kind e2e.
	KUBECONFIG=$(KIND_KUBECONFIG) $(KUBECTL) --context kind-$(KIND_CLUSTER) apply --server-side --force-conflicts -f $(LOCALBIN)/aso-crds/
	@if KUBECONFIG=$(KIND_KUBECONFIG) $(KUBECTL) --context kind-$(KIND_CLUSTER) get crd vaults.keyvault.azure.com >/dev/null 2>&1; then \
		KUBECONFIG=$(KIND_KUBECONFIG) $(KUBECTL) --context kind-$(KIND_CLUSTER) patch crd vaults.keyvault.azure.com --type='json' -p='[{"op":"replace","path":"/spec/conversion","value":{"strategy":"None"}}]'; \
	fi
	@if KUBECONFIG=$(KIND_KUBECONFIG) $(KUBECTL) --context kind-$(KIND_CLUSTER) get crd roleassignments.authorization.azure.com >/dev/null 2>&1; then \
		KUBECONFIG=$(KIND_KUBECONFIG) $(KUBECTL) --context kind-$(KIND_CLUSTER) patch crd roleassignments.authorization.azure.com --type='json' -p='[{"op":"replace","path":"/spec/conversion","value":{"strategy":"None"}}]'; \
	fi

.PHONY: install-dis-identity-crd-kind
install-dis-identity-crd-kind: setup-test-e2e dis-identity-crd ## Install ApplicationIdentity CRD for Kind e2e.
	KUBECONFIG=$(KIND_KUBECONFIG) $(KUBECTL) --context kind-$(KIND_CLUSTER) apply --server-side --force-conflicts -f $(DIS_IDENTITY_CRD_FILE)

.PHONY: install-external-secrets-crd-kind
install-external-secrets-crd-kind: setup-test-e2e external-secrets-crd ## Install External Secrets SecretStore CRD for Kind e2e.
	KUBECONFIG=$(KIND_KUBECONFIG) $(KUBECTL) --context kind-$(KIND_CLUSTER) apply --server-side --force-conflicts -f $(EXTERNAL_SECRETS_CRD_FILE)

.PHONY: seed-dis-identity-kind
seed-dis-identity-kind: setup-test-e2e ## Seed ApplicationIdentity and ready status for sample-based Kind e2e.
	KUBECONFIG=$(KIND_KUBECONFIG) $(KUBECTL) --context kind-$(KIND_CLUSTER) apply -f config/kind/applicationidentities.yaml
	KUBECONFIG=$(KIND_KUBECONFIG) $(KUBECTL) --context kind-$(KIND_CLUSTER) patch applicationidentity app-identity-sample -n default \
		--subresource=status --type=merge \
		-p '{"status":{"managedIdentityName":"app-identity-sample-managed","principalId":"app-identity-sample-principal","conditions":[{"type":"Ready","status":"True","reason":"Ready","message":"Seeded for Kind e2e","lastTransitionTime":"2026-01-01T00:00:00Z"}]}}'

.PHONY: test-e2e-kind
test-e2e-kind: cleanup-test-e2e setup-test-e2e docker-build kind-load install install-aso-crds-kind install-dis-identity-crd-kind install-external-secrets-crd-kind deploy-kind ## Setup Kind e2e env and apply samples.
	$(MAKE) seed-dis-identity-kind
	KUBECONFIG=$(KIND_KUBECONFIG) $(KUBECTL) --context kind-$(KIND_CLUSTER) apply -k config/samples

.PHONY: test-e2e-kind-ci
test-e2e-kind-ci: cleanup-test-e2e setup-test-e2e manifests generate fmt vet install-aso-crds-kind install-dis-identity-crd-kind install-external-secrets-crd-kind ## Run the e2e tests on Kind and cleanup (CI).
	@test_status=0; cleanup_status=0; \
	KUBECONFIG=$(KIND_KUBECONFIG) CERT_MANAGER_INSTALL_SKIP=$(CERT_MANAGER_INSTALL_SKIP) KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -ginkgo.v || test_status=$$?; \
	$(MAKE) cleanup-test-e2e || cleanup_status=$$?; \
	if [ $$test_status -ne 0 ]; then exit $$test_status; fi; \
	exit $$cleanup_status

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)
	@rm -f "$(KIND_KUBECONFIG)"

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	"$(GOLANGCI_LINT)" run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	"$(GOLANGCI_LINT)" run --fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	"$(GOLANGCI_LINT)" config verify

.PHONY: govulncheck
govulncheck: govulncheck-bin ## Run govulncheck against code and dependencies.
	"$(GOVULNCHECK)" ./...

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager ./cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name dis-vault-operator-builder
	$(CONTAINER_TOOL) buildx use dis-vault-operator-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm dis-vault-operator-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	@set -e; \
		kustomize_file="config/manager/kustomization.yaml"; \
		kustomize_backup="$$(mktemp)"; \
		cp "$$kustomize_file" "$$kustomize_backup"; \
		trap 'cp "$$kustomize_backup" "$$kustomize_file"; rm -f "$$kustomize_backup"' EXIT; \
		mkdir -p dist; \
		( cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG} ); \
		"$(KUSTOMIZE)" build config/default > dist/install.yaml

##@ Deployment

## Kind load target to load the controller image into a kind cluster.
.PHONY: kind-load
kind-load: setup-test-e2e
	$(CONTAINER_TOOL) save -o $(KIND_IMAGE_ARCHIVE) $(IMG)
	$(KIND) load image-archive --name $(KIND_CLUSTER) $(KIND_IMAGE_ARCHIVE)

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" apply -f -; else echo "No CRDs to install; skipping."; fi

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@out="$$( "$(KUSTOMIZE)" build config/crd 2>/dev/null || true )"; \
	if [ -n "$$out" ]; then echo "$$out" | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -; else echo "No CRDs to delete; skipping."; fi

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	@set -e; \
		kustomize_file="config/manager/kustomization.yaml"; \
		kustomize_backup="$$(mktemp)"; \
		cp "$$kustomize_file" "$$kustomize_backup"; \
		trap 'cp "$$kustomize_backup" "$$kustomize_file"; rm -f "$$kustomize_backup"' EXIT; \
		( cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG} ); \
		"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" apply -f -

.PHONY: deploy-kind
deploy-kind: setup-test-e2e manifests kustomize ## Deploy controller to a Kind cluster (IfNotPresent pull policy).
	@set -e; \
		kustomize_file="config/manager/kustomization.yaml"; \
		kustomize_backup="$$(mktemp)"; \
		cp "$$kustomize_file" "$$kustomize_backup"; \
		trap 'cp "$$kustomize_backup" "$$kustomize_file"; rm -f "$$kustomize_backup"' EXIT; \
		( cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG} ); \
		"$(KUSTOMIZE)" build config/kind | KUBECONFIG=$(KIND_KUBECONFIG) "$(KUBECTL)" --context kind-$(KIND_CLUSTER) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	"$(KUSTOMIZE)" build config/default | "$(KUBECTL)" delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p "$(LOCALBIN)"

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
GOVULNCHECK ?= $(LOCALBIN)/govulncheck

## Tool Versions
KUSTOMIZE_VERSION ?= v5.7.1
CONTROLLER_TOOLS_VERSION ?= v0.19.0

#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell v='$(call gomodver,sigs.k8s.io/controller-runtime)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_VERSION manually (controller-runtime replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?([0-9]+)\.([0-9]+).*/release-\1.\2/')

#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell v='$(call gomodver,k8s.io/api)'; \
  [ -n "$$v" ] || { echo "Set ENVTEST_K8S_VERSION manually (k8s.io/api replace has no tag)" >&2; exit 1; }; \
  printf '%s\n' "$$v" | sed -E 's/^v?[0-9]+\.([0-9]+).*/1.\1/')

GOLANGCI_LINT_VERSION ?= v2.7.2
GOVULNCHECK_VERSION ?= latest
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: aso-crds dis-identity-crd external-secrets-crd envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@"$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: govulncheck-bin
govulncheck-bin: $(GOVULNCHECK) ## Download govulncheck locally if necessary.
$(GOVULNCHECK): $(LOCALBIN)
	$(call go-install-tool,$(GOVULNCHECK),golang.org/x/vuln/cmd/govulncheck,$(GOVULNCHECK_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f "$(1)" ;\
GOBIN="$(LOCALBIN)" go install $${package} ;\
mv "$(LOCALBIN)/$$(basename "$(1)")" "$(1)-$(3)" ;\
} ;\
ln -sf "$$(realpath "$(1)-$(3)")" "$(1)"
endef

define gomodver
$(shell go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' $(1) 2>/dev/null)
endef

ASO_CRD_VERSION ?= v2.17.0
DIS_IDENTITY_CRD_NAME ?= application.dis.altinn.cloud_applicationidentities.yaml
DIS_IDENTITY_MODULE ?= github.com/Altinn/altinn-platform/services/dis-identity-operator
DIS_IDENTITY_CRD_REL_PATH ?= config/crd/bases/$(DIS_IDENTITY_CRD_NAME)
DIS_IDENTITY_CRD_LOCAL_FILE ?= $(WORKSPACE_ROOT)/services/dis-identity-operator/$(DIS_IDENTITY_CRD_REL_PATH)
DIS_IDENTITY_CRD_DIR ?= $(LOCALBIN)/dis-identity-crds
DIS_IDENTITY_CRD_FILE ?= $(DIS_IDENTITY_CRD_DIR)/$(DIS_IDENTITY_CRD_NAME)
EXTERNAL_SECRETS_VERSION ?= v2.0.1
EXTERNAL_SECRETS_CRD_NAME ?= external-secrets.io_secretstores.yaml
EXTERNAL_SECRETS_CRD_REL_PATH ?= config/crds/bases/$(EXTERNAL_SECRETS_CRD_NAME)
EXTERNAL_SECRETS_CRD_DIR ?= $(LOCALBIN)/external-secrets-crds
EXTERNAL_SECRETS_CRD_FILE ?= $(EXTERNAL_SECRETS_CRD_DIR)/external-secrets.io_secretstores_$(EXTERNAL_SECRETS_VERSION).yaml
EXTERNAL_SECRETS_CRD_ENVTEST_DIR ?= $(EXTERNAL_SECRETS_CRD_DIR)/current
EXTERNAL_SECRETS_CRD_ENVTEST_FILE ?= $(EXTERNAL_SECRETS_CRD_ENVTEST_DIR)/$(EXTERNAL_SECRETS_CRD_NAME)

.PHONY: aso-crds
aso-crds: ## Download the ASO CRDs locally if necessary.
	@mkdir -p $(LOCALBIN)/aso-crds
	@if [ -f "$(LOCALBIN)/aso-crds/azureserviceoperator_customresourcedefinitions_$(ASO_CRD_VERSION).yaml" ]; then \
		echo "ASO CRDs already present; skipping download."; \
	else \
		curl -fsSL https://github.com/Azure/azure-service-operator/releases/download/$(ASO_CRD_VERSION)/azureserviceoperator_customresourcedefinitions_$(ASO_CRD_VERSION).yaml -o $(LOCALBIN)/aso-crds/azureserviceoperator_customresourcedefinitions_$(ASO_CRD_VERSION).yaml; \
	fi

.PHONY: dis-identity-crd
dis-identity-crd: ## Download the ApplicationIdentity CRD locally if necessary.
	@mkdir -p $(DIS_IDENTITY_CRD_DIR)
	@if [ -f "$(DIS_IDENTITY_CRD_LOCAL_FILE)" ]; then \
		cp "$(DIS_IDENTITY_CRD_LOCAL_FILE)" "$(DIS_IDENTITY_CRD_FILE)"; \
		echo "ApplicationIdentity CRD copied from local workspace: $(DIS_IDENTITY_CRD_LOCAL_FILE)"; \
	else \
		module_dir="$$(go list -m -f '{{.Dir}}' $(DIS_IDENTITY_MODULE) 2>/dev/null || true)"; \
		if [ -z "$$module_dir" ] || [ ! -f "$$module_dir/$(DIS_IDENTITY_CRD_REL_PATH)" ]; then \
			go mod download $(DIS_IDENTITY_MODULE) >/dev/null; \
			module_dir="$$(go list -m -f '{{.Dir}}' $(DIS_IDENTITY_MODULE) 2>/dev/null || true)"; \
		fi; \
		if [ -n "$$module_dir" ] && [ -f "$$module_dir/$(DIS_IDENTITY_CRD_REL_PATH)" ]; then \
			cp "$$module_dir/$(DIS_IDENTITY_CRD_REL_PATH)" "$(DIS_IDENTITY_CRD_FILE)"; \
			echo "ApplicationIdentity CRD copied from module source: $$module_dir/$(DIS_IDENTITY_CRD_REL_PATH)"; \
		elif [ -f "$(DIS_IDENTITY_CRD_FILE)" ]; then \
			echo "ApplicationIdentity CRD already present; skipping download."; \
		else \
			echo "Failed to resolve ApplicationIdentity CRD from workspace or module source." >&2; \
			exit 1; \
		fi; \
	fi

.PHONY: external-secrets-crd
external-secrets-crd: ## Download the External Secrets SecretStore CRD locally if necessary.
	@mkdir -p $(EXTERNAL_SECRETS_CRD_DIR)
	@mkdir -p $(EXTERNAL_SECRETS_CRD_ENVTEST_DIR)
	@if [ -f "$(EXTERNAL_SECRETS_CRD_FILE)" ]; then \
		echo "External Secrets CRD already present; skipping download."; \
	else \
		curl -fsSL https://raw.githubusercontent.com/external-secrets/external-secrets/$(EXTERNAL_SECRETS_VERSION)/$(EXTERNAL_SECRETS_CRD_REL_PATH) -o "$(EXTERNAL_SECRETS_CRD_FILE)"; \
	fi
	@cp "$(EXTERNAL_SECRETS_CRD_FILE)" "$(EXTERNAL_SECRETS_CRD_ENVTEST_FILE)"
