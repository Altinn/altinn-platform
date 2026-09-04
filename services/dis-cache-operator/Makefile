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
