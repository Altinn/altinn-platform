# flux-dispatch is a plain net/http Go service (not a kubebuilder operator). It
# reuses the shared ../../Makefile.common for the generic Go tasks (fmt, vet,
# lint, build, docker, tooling) and overrides the build/test wiring so none of
# the kubebuilder codegen/envtest machinery runs.

APP_NAME ?= flux-dispatch
IMG ?= $(APP_NAME):local
CONTAINER_TOOL ?= podman

# flux-dispatch is a self-contained module with no sibling dependencies. The
# operators' `make workspace` may leave a root go.work that does not list this
# service, which would break `go` commands here; ignore any ambient workspace.
# No-op in CI (no go.work present).
export GOWORK := off

# Plain-service build/test wiring (no controller-gen / envtest).
BINARY := $(APP_NAME)
MAIN := cmd/main.go
BUILD_DEPS := fmt vet
TEST_DEPS := fmt vet
GO_TEST_CMD := go test -race ./... -coverprofile cover.out

include ../../Makefile.common

##@ flux-dispatch

.PHONY: verify
verify: ## Run the gate the PR must pass: gofmt, go vet, race tests.
	@echo ">> gofmt -l ."
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "$$out"; exit 1; fi
	go vet ./...
	go test -race ./...

# cdk8s-manifests / cdk8s-manifests-verify deliberately are not named
# manifests / manifests-verify (the names services/lakmus uses for the same
# cdk8s generate-and-commit pattern): Makefile.common already defines
# `manifests: controller-gen` for the kubebuilder operators, and GNU Make
# unions prerequisites across same-named rules even when the recipe is
# overridden — so reusing that name here would silently pull in
# controller-gen on every `make manifests`, exactly the kubebuilder machinery
# this service opts out of above.
CONFIG_DIR := config

.PHONY: cdk8s-manifests
cdk8s-manifests: ## Synthesize Kubernetes manifests into config/.
	@echo ">> Synthesizing Kubernetes manifests into $(CONFIG_DIR)"
	go run ./manifests

.PHONY: cdk8s-manifests-verify
cdk8s-manifests-verify: cdk8s-manifests ## Re-generate manifests and fail if config/ changed.
	@echo ">> Verifying generated manifests are committed under $(CONFIG_DIR)"
	@if [ -n "$$(git status --porcelain -- "$(CONFIG_DIR)")" ]; then \
		echo "Generated manifests are out of date. Run 'make cdk8s-manifests' and commit config changes."; \
		git --no-pager diff -- "$(CONFIG_DIR)"; \
		exit 1; \
	fi

# tidy is already provided by Makefile.common (go mod tidy); no override needed.
