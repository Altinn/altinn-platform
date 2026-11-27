# dis-messaging-canary is a plain Go service, not a kubebuilder operator. It
# uses the shared ../../Makefile.common and overrides the build and test wiring,
# so that the controller-gen and envtest machinery does not run.

APP_NAME ?= dis-messaging-canary
IMG ?= $(APP_NAME):local
CONTAINER_TOOL ?= podman

# The module has no sibling dependencies. Ignore a root go.work that does not
# list this service.
export GOWORK := off

BINARY := $(APP_NAME)
BUILD_DEPS := fmt vet
TEST_DEPS := fmt vet
GO_TEST_CMD := go test ./... -coverprofile cover.out

include ../../Makefile.common
