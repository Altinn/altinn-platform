APP_NAME ?= dis-messaging-operator
# Keep the localhost/ prefix: in local Kind flows we load this image directly,
# and this avoids accidental registry lookups when the image name is overridden.
IMG ?= localhost/controller:latest
CONTAINER_TOOL ?= podman

# Consumed by Makefile.common: operator CI check suite and local tooling bootstrap.
RUN_CHECKS := fmt generate manifests test lint
SETUP_TOOLS := golangci-lint kustomize controller-gen setup-envtest

include ../../Makefile.common

##@ Messaging-specific

ASO_MODULE ?= github.com/Azure/azure-service-operator/v2
ASO_CRD_DIR ?= $(LOCALBIN)/aso-crds
ASO_CRDS ?= namespacesqueues.servicebus.azure.com roleassignments.authorization.azure.com

# The envtest controller suite needs the ASO and ApplicationIdentity CRDs on
# disk. This rule has no recipe. It adds aso-crds and dis-identity-crd as
# prerequisites of the shared setup-envtest target and keeps its recipe.
setup-envtest: aso-crds dis-identity-crd

# ASO publishes its CRDs only as one release bundle. envtest would install
# every CRD in it, so this copies out the ones the operator writes.
.PHONY: aso-crds
aso-crds: ## Copy the ASO CRDs of the go.mod version from the ASO release bundle for envtest.
	@set -e; mkdir -p "$(ASO_CRD_DIR)"; \
	version="$$(GOWORK=off go list -m -f '{{.Version}}' $(ASO_MODULE))"; \
	bundle="$(LOCALBIN)/aso-crds-$$version.yaml"; \
	if [ ! -f "$$bundle" ]; then \
		curl -fsSL -o "$$bundle.tmp" "https://github.com/Azure/azure-service-operator/releases/download/$$version/azureserviceoperator_customresourcedefinitions_$$version.yaml"; \
		mv "$$bundle.tmp" "$$bundle"; \
	fi; \
	for crd in $(ASO_CRDS); do \
		out="$(ASO_CRD_DIR)/$$crd.yaml"; \
		grep -qx "    app.kubernetes.io/version: $$version" "$$out" 2>/dev/null && continue; \
		awk -v name="$$crd" '/^---$$/ { if (found) for (i = 1; i <= n; i++) print lines[i]; n = 0; found = 0; next } \
			{ lines[++n] = $$0 } $$0 == "  name: " name { found = 1 } \
			END { if (found) for (i = 1; i <= n; i++) print lines[i] }' "$$bundle" > "$$out"; \
		[ -s "$$out" ] || { rm -f "$$out"; echo "CRD $$crd not found in $$bundle" >&2; exit 1; }; \
	done

DIS_IDENTITY_MODULE ?= github.com/Altinn/altinn-platform/services/dis-identity-operator
DIS_IDENTITY_CRD ?= config/crd/bases/application.dis.altinn.cloud_applicationidentities.yaml
DIS_IDENTITY_CRD_DIR ?= $(LOCALBIN)/dis-identity-crds

.PHONY: dis-identity-crd
dis-identity-crd: ## Copy the ApplicationIdentity CRD of the go.mod version from the Go module cache for envtest.
	@set -e; mkdir -p "$(DIS_IDENTITY_CRD_DIR)"; \
	GOWORK=off go mod download $(DIS_IDENTITY_MODULE); \
	dir="$$(GOWORK=off go list -m -f '{{.Dir}}' $(DIS_IDENTITY_MODULE))"; \
	install -m 0644 "$$dir/$(DIS_IDENTITY_CRD)" "$(DIS_IDENTITY_CRD_DIR)/"

WORKSPACE_ROOT ?= $(abspath $(CURDIR)/../..)

# Local dev also initialises the go.work workspace so builds resolve this
# module when a checkout carries a dev go.work.
setup-local-env: workspace

.PHONY: workspace
workspace: ## Add dis-messaging-operator to the local dev go.work (not committed).
	@cd "$(WORKSPACE_ROOT)" && \
	if [ -f go.work ]; then \
		echo "go.work already exists; updating modules"; \
		go work use ./services/dis-messaging-operator; \
	else \
		GOWORK="$(WORKSPACE_ROOT)/go.work" go work init ./services/dis-messaging-operator; \
	fi
	@echo "Local workspace at $(WORKSPACE_ROOT)/go.work (ignored by git)."

.PHONY: workspace-clean
workspace-clean: ## Remove local go.work/go.work.sum (dev only).
	@rm -f "$(WORKSPACE_ROOT)/go.work" "$(WORKSPACE_ROOT)/go.work.sum"
