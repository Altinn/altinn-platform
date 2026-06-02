# dis-console is a plain net/http Go service (not a kubebuilder operator). It
# reuses the shared ../../Makefile.common for the generic Go tasks (fmt, vet,
# lint, build, docker, tooling, cache, checks) and overrides the build/test
# wiring so none of the kubebuilder codegen/envtest machinery runs.

APP_NAME ?= dis-console
IMG ?= $(APP_NAME):local
CONTAINER_TOOL ?= podman

# Plain-service build/test wiring (no controller-gen / envtest).
BINARY := $(APP_NAME)
MAIN := cmd/main.go
BUILD_DEPS := fmt vet
TEST_DEPS := fmt vet
GO_TEST_CMD := go test ./... -coverprofile cover.out
RUN_ARGS := --local

include ../../Makefile.common
