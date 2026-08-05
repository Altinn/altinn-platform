APP_NAME ?= dis-vault-operator
# Keep the localhost/ prefix: in local Kind flows we load this image directly,
# and this avoids accidental registry lookups when the image name is overridden.
IMG ?= localhost/controller:latest
CONTAINER_TOOL ?= podman

# e2e: skip cert-manager install; injected into Makefile.common's test-e2e via E2E_ENV.
CERT_MANAGER_INSTALL_SKIP ?= true
E2E_ENV := CERT_MANAGER_INSTALL_SKIP=$(CERT_MANAGER_INSTALL_SKIP)

# Consumed by Makefile.common: operator CI check suite and local tooling bootstrap.
RUN_CHECKS := fmt generate manifests test-ci lint
SETUP_TOOLS := golangci-lint kustomize controller-gen setup-envtest

include ../../Makefile.common

##@ Vault-specific

WORKSPACE_ROOT ?= $(abspath $(CURDIR)/../..)
KIND_IMAGE_ARCHIVE := bin/dis-vault-operator_image.tar

# The envtest controller suite needs the ASO, ApplicationIdentity and External
# Secrets CRDs on disk. These recipe-less rules add them as prerequisites of the
# shared setup-envtest target without overriding its recipe.
setup-envtest: aso-crds dis-identity-crd external-secrets-crd

# Local dev also initialises the go.work workspace (vault + identity).
setup-local-env: workspace

.PHONY: workspace
workspace: ## Create local go.work for dev across dis-vault-operator and dis-identity-operator (not committed).
	@cd "$(WORKSPACE_ROOT)" && \
	if [ -f go.work ]; then \
		echo "go.work already exists; updating modules"; \
		go work use ./services/dis-vault-operator ./services/dis-identity-operator; \
	else \
		go work init ./services/dis-vault-operator ./services/dis-identity-operator; \
	fi
	@echo "Local workspace at $(WORKSPACE_ROOT)/go.work (ignored by git)."

.PHONY: workspace-clean
workspace-clean: ## Remove local go.work/go.work.sum (dev only).
	@rm -f "$(WORKSPACE_ROOT)/go.work" "$(WORKSPACE_ROOT)/go.work.sum"

.PHONY: clean
clean: workspace-clean tidy verify-deps lint test ## Clean local dev artifacts before pushing.

.PHONY: verify-deps
verify-deps: ## Verify go.mod/go.sum resolve from a clean module cache.
	@tmp_modcache="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp_modcache"' EXIT INT TERM; \
	echo "Verifying module pins with clean module cache: $$tmp_modcache"; \
	GOWORK=off GOFLAGS=-mod=readonly GOMODCACHE="$$tmp_modcache" go mod download

.PHONY: test-ci
test-ci: manifests generate fmt vet setup-envtest ## Run tests (CI style).
	DISVAULT_SKIP_ENVTEST=1 KUBEBUILDER_ASSETS="$(shell "$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: govulncheck
govulncheck: govulncheck-bin ## Run govulncheck against code and dependencies.
	"$(GOVULNCHECK)" ./...

##@ Kind e2e

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
test-e2e-kind: export KUBECONFIG = $(KIND_KUBECONFIG)
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

.PHONY: kind-load
kind-load: setup-test-e2e
	$(CONTAINER_TOOL) save -o $(KIND_IMAGE_ARCHIVE) $(IMG)
	$(KIND) load image-archive --name $(KIND_CLUSTER) $(KIND_IMAGE_ARCHIVE)

.PHONY: deploy-kind
deploy-kind: setup-test-e2e manifests kustomize ## Deploy controller to a Kind cluster (IfNotPresent pull policy).
	@set -e; \
		kustomize_file="config/manager/kustomization.yaml"; \
		kustomize_backup="$$(mktemp)"; \
		cp "$$kustomize_file" "$$kustomize_backup"; \
		trap 'cp "$$kustomize_backup" "$$kustomize_file"; rm -f "$$kustomize_backup"' EXIT; \
		( cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG} ); \
		"$(KUSTOMIZE)" build config/kind | KUBECONFIG=$(KIND_KUBECONFIG) "$(KUBECTL)" --context kind-$(KIND_CLUSTER) apply -f -

##@ Tool + CRD downloads

GOVULNCHECK ?= $(LOCALBIN)/govulncheck
GOVULNCHECK_VERSION ?= latest
.PHONY: govulncheck-bin
govulncheck-bin: $(GOVULNCHECK) ## Download govulncheck locally if necessary.
$(GOVULNCHECK): $(LOCALBIN)
	$(call go-install-tool,$(GOVULNCHECK),golang.org/x/vuln/cmd/govulncheck,$(GOVULNCHECK_VERSION))

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
		module_dir="$$(GOWORK=off go list -m -f '{{.Dir}}' $(DIS_IDENTITY_MODULE) 2>/dev/null || true)"; \
		if [ -z "$$module_dir" ] || [ ! -f "$$module_dir/$(DIS_IDENTITY_CRD_REL_PATH)" ]; then \
			GOWORK=off go mod download $(DIS_IDENTITY_MODULE) >/dev/null; \
			module_dir="$$(GOWORK=off go list -m -f '{{.Dir}}' $(DIS_IDENTITY_MODULE) 2>/dev/null || true)"; \
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
