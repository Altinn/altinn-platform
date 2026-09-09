APP_NAME ?= dis-cache-operator
# Keep the localhost/ prefix: in local Kind flows we load this image directly,
# and this avoids accidental registry lookups when the image name is overridden.
IMG ?= localhost/controller:latest
CONTAINER_TOOL ?= podman

# Consumed by Makefile.common: operator CI check suite and local tooling bootstrap.
RUN_CHECKS := fmt generate manifests test lint
SETUP_TOOLS := golangci-lint kustomize controller-gen setup-envtest

include ../../Makefile.common

##@ Cache-specific

VALKEY_OPERATOR_MODULE ?= github.com/valkey-io/valkey-operator
VALKEY_CRD_DIR ?= $(LOCALBIN)/valkey-crds

# The envtest controller suite needs the ValkeyCluster CRDs on disk. This
# recipe-less rule adds them as a prerequisite of the shared setup-envtest
# target without overriding its recipe.
setup-envtest: valkey-crds

.PHONY: valkey-crds
valkey-crds: ## Copy the valkey-operator CRDs from the Go module cache for envtest.
	@mkdir -p "$(VALKEY_CRD_DIR)"
	@module_dir="$$(GOWORK=off go list -m -f '{{.Dir}}' $(VALKEY_OPERATOR_MODULE))"; \
	cp -f "$$module_dir"/config/crd/bases/valkey.io_*.yaml "$(VALKEY_CRD_DIR)/" && chmod u+w "$(VALKEY_CRD_DIR)"/*.yaml

WORKSPACE_ROOT ?= $(abspath $(CURDIR)/../..)

# Local dev also initialises the go.work workspace so builds resolve this
# module when a checkout carries a dev go.work.
setup-local-env: workspace

.PHONY: workspace
workspace: ## Add dis-cache-operator to the local dev go.work (not committed).
	@cd "$(WORKSPACE_ROOT)" && \
	if [ -f go.work ]; then \
		echo "go.work already exists; updating modules"; \
		go work use ./services/dis-cache-operator; \
	else \
		GOWORK="$(WORKSPACE_ROOT)/go.work" go work init ./services/dis-cache-operator; \
	fi
	@echo "Local workspace at $(WORKSPACE_ROOT)/go.work (ignored by git)."

.PHONY: workspace-clean
workspace-clean: ## Remove local go.work/go.work.sum (dev only).
	@rm -f "$(WORKSPACE_ROOT)/go.work" "$(WORKSPACE_ROOT)/go.work.sum"
