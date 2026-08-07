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

.PHONY: tidy
tidy: ## Sync go.mod/go.sum.
	go mod tidy
