APP_NAME ?= dis-pgsql-operator
# Keep the localhost/ prefix: in local Kind flows we load this image directly,
# and this avoids accidental registry lookups when the image name is overridden.
IMG ?= localhost/controller:latest
CONTAINER_TOOL ?= podman

# e2e: skip cert-manager install and set the go test timeout; injected into
# Makefile.common's test-e2e via the E2E_ENV / E2E_GOTEST_FLAGS hooks.
CERT_MANAGER_INSTALL_SKIP ?= true
E2E_TEST_TIMEOUT ?= 30m
E2E_ENV := CERT_MANAGER_INSTALL_SKIP=$(CERT_MANAGER_INSTALL_SKIP)
E2E_GOTEST_FLAGS := -timeout=$(E2E_TEST_TIMEOUT)

# Consumed by Makefile.common: operator CI check suite and local tooling bootstrap.
RUN_CHECKS := fmt generate manifests test-ci lint
SETUP_TOOLS := golangci-lint kustomize controller-gen setup-envtest

include ../../Makefile.common

##@ Pgsql-specific

WORKSPACE_ROOT ?= $(abspath $(CURDIR)/../..)
KIND_IMAGE_ARCHIVE := bin/dis-pgsql-operator_image.tar

# Operator config defaults (overridable via the environment for local runs).
WRITE_NAMESPACE ?= default
AZURE_SUBSCRIPTION_ID ?= 00000000-0000-0000-0000-000000000000
AZURE_TENANT_ID ?= 00000000-0000-0000-0000-000000000000
AZURE_VNET_RESOURCE_GROUP ?= rg-dis-dev-network
AZURE_VNET_NAME ?= dis-dev-vnet
DB_VNET_NAME ?= dis-dev-vnet
AKS_VNET_NAME ?= dis-dev-aks-vnet
USE_AZ_FAKES ?= true

# The envtest controller suite needs the ASO and ApplicationIdentity CRDs on
# disk. These recipe-less rules add them as prerequisites of the shared
# setup-envtest target without overriding its recipe.
setup-envtest: aso-crds dis-identity-crd

# Local dev also initialises the go.work workspace (pgsql + identity).
setup-local-env: workspace

.PHONY: workspace
workspace: ## Create local go.work for dev across dis-pgsql-operator and dis-identity-operator (not committed).
	@cd "$(WORKSPACE_ROOT)" && \
	if [ -f go.work ]; then \
		echo "go.work already exists; updating modules"; \
		go work use ./services/dis-pgsql-operator ./services/dis-identity-operator; \
	else \
		go work init ./services/dis-pgsql-operator ./services/dis-identity-operator; \
	fi
	@echo "Local workspace at $(WORKSPACE_ROOT)/go.work (ignored by git)."

.PHONY: workspace-clean
workspace-clean: ## Remove local go.work/go.work.sum (dev only).
	@rm -f "$(WORKSPACE_ROOT)/go.work" "$(WORKSPACE_ROOT)/go.work.sum"

.PHONY: clean
clean: restore-kustomize workspace-clean tidy test ## Clean local dev artifacts before pushing.

.PHONY: restore-kustomize
restore-kustomize: ## Restore kustomize edits made by deploy-kind.
	git restore --source=HEAD --staged --worktree -- config/manager/kustomization.yaml

.PHONY: test-ci
test-ci: manifests generate fmt vet envtest ## Run tests (envtest disabled).
	DISPG_SKIP_ENVTEST=1 KUBEBUILDER_ASSETS="$$("$(ENVTEST)" use $(ENVTEST_K8S_VERSION) --bin-dir "$(LOCALBIN)" -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

##@ Kind e2e

.PHONY: test-e2e-kind
test-e2e-kind: export KUBECONFIG = $(KIND_KUBECONFIG)
test-e2e-kind: cleanup-test-e2e setup-test-e2e docker-build kind-load install install-aso-crds-kind install-dis-identity-crd-kind deploy-kind
	kubectl apply -f config/samples/storage_v1alpha1_databaseserver.yaml
	$(MAKE) seed-dis-identity-kind

.PHONY: test-e2e-kind-ci
test-e2e-kind-ci: cleanup-test-e2e setup-test-e2e manifests generate fmt vet install-aso-crds-kind install-dis-identity-crd-kind ## Run the e2e tests on Kind and cleanup (CI).
	KUBECONFIG="$(KIND_KUBECONFIG)" CERT_MANAGER_INSTALL_SKIP=$(CERT_MANAGER_INSTALL_SKIP) KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -ginkgo.v -timeout=$(E2E_TEST_TIMEOUT)
	$(MAKE) cleanup-test-e2e

.PHONY: install-aso-crds-kind
install-aso-crds-kind: export KUBECONFIG = $(KIND_KUBECONFIG)
install-aso-crds-kind: aso-crds
	kubectl apply --server-side --force-conflicts -f bin/aso-crds/

	# Patch ASO CRDs to disable conversion webhooks for Kind e2e tests
	kubectl patch crd privatednszones.network.azure.com \
	--type='json' \
	-p='[{"op":"replace","path":"/spec/conversion","value":{"strategy":"None"}}]'

	kubectl patch crd privatednszonesvirtualnetworklinks.network.azure.com \
	--type='json' \
	-p='[{"op":"replace","path":"/spec/conversion","value":{"strategy":"None"}}]'

	kubectl patch crd flexibleservers.dbforpostgresql.azure.com \
	--type='json' \
	-p='[{"op":"replace","path":"/spec/conversion","value":{"strategy":"None"}}]'

	kubectl patch crd flexibleserversconfigurations.dbforpostgresql.azure.com \
	--type='json' \
	-p='[{"op":"replace","path":"/spec/conversion","value":{"strategy":"None"}}]'

	kubectl patch crd flexibleserversadministrators.dbforpostgresql.azure.com \
	--type='json' \
	-p='[{"op":"replace","path":"/spec/conversion","value":{"strategy":"None"}}]'

	kubectl patch crd flexibleserversdatabases.dbforpostgresql.azure.com \
	--type='json' \
	-p='[{"op":"replace","path":"/spec/conversion","value":{"strategy":"None"}}]'

	kubectl patch crd roleassignments.authorization.azure.com \
	--type='json' \
	-p='[{"op":"replace","path":"/spec/conversion","value":{"strategy":"None"}}]'

.PHONY: install-dis-identity-crd-kind
install-dis-identity-crd-kind: export KUBECONFIG = $(KIND_KUBECONFIG)
install-dis-identity-crd-kind: dis-identity-crd
	kubectl apply --server-side --force-conflicts -f $(DIS_IDENTITY_CRD_FILE)

.PHONY: seed-dis-identity-kind
seed-dis-identity-kind: export KUBECONFIG = $(KIND_KUBECONFIG)
seed-dis-identity-kind: ## Apply ApplicationIdentity stubs and patch status for Kind tests.
	kubectl apply -f config/kind/applicationidentities.yaml
	kubectl patch applicationidentity adminidentity -n default \
		--subresource=status --type=merge \
		-p '{"status":{"managedIdentityName":"adminidentity","principalId":"adminidentity-principal-id"}}'
	kubectl patch applicationidentity useridentity -n default \
		--subresource=status --type=merge \
		-p '{"status":{"managedIdentityName":"user1","principalId":"user1-principal-id"}}'
	kubectl patch applicationidentity useridentity2 -n default \
		--subresource=status --type=merge \
		-p '{"status":{"managedIdentityName":"user2","principalId":"user2-principal-id"}}'

.PHONY: kind-load
kind-load:
	$(CONTAINER_TOOL) save -o $(KIND_IMAGE_ARCHIVE) $(IMG)
	kind load image-archive --name $(KIND_CLUSTER) $(KIND_IMAGE_ARCHIVE)

.PHONY: deploy-kind
deploy-kind: export KUBECONFIG = $(KIND_KUBECONFIG)
deploy-kind: manifests kustomize ## Deploy controller to a Kind cluster (IfNotPresent pull policy).
	@set -e; \
		trap '$(MAKE) -C "$(CURDIR)" restore-kustomize >/dev/null' EXIT; \
		( cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG} ); \
		"$(KUSTOMIZE)" build config/kind | "$(KUBECTL)" apply -f -

##@ CRD downloads

ASO_CRD_VERSION ?= v2.17.0
DIS_IDENTITY_CRD_NAME ?= application.dis.altinn.cloud_applicationidentities.yaml
DIS_IDENTITY_MODULE ?= github.com/Altinn/altinn-platform/services/dis-identity-operator
DIS_IDENTITY_CRD_REL_PATH ?= config/crd/bases/$(DIS_IDENTITY_CRD_NAME)
DIS_IDENTITY_CRD_LOCAL_FILE ?= $(WORKSPACE_ROOT)/services/dis-identity-operator/$(DIS_IDENTITY_CRD_REL_PATH)
DIS_IDENTITY_CRD_DIR ?= $(LOCALBIN)/dis-identity-crds
DIS_IDENTITY_CRD_FILE ?= $(DIS_IDENTITY_CRD_DIR)/$(DIS_IDENTITY_CRD_NAME)

.PHONY: aso-crds
aso-crds: ## Download the ASO CRDs locally if necessary.
	@mkdir -p $(LOCALBIN)/aso-crds
	@if [ -f "$(LOCALBIN)/aso-crds/azureserviceoperator_customresourcedefinitions_$(ASO_CRD_VERSION).yaml" ]; then \
		echo "ASO CRDs already present; skipping download."; \
	else \
		curl -sSL https://github.com/Azure/azure-service-operator/releases/download/$(ASO_CRD_VERSION)/azureserviceoperator_customresourcedefinitions_$(ASO_CRD_VERSION).yaml -o $(LOCALBIN)/aso-crds/azureserviceoperator_customresourcedefinitions_$(ASO_CRD_VERSION).yaml; \
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
