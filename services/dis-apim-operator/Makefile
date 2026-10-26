APP_NAME ?= dis-apim-operator
IMG ?= controller:latest
CONTAINER_TOOL ?= podman

# Tool versions matching the kubebuilder v4.16.0 scaffold (override the
# Makefile.common defaults). Keep GOLANGCI_LINT_VERSION in sync with
# .custom-gcl.yml and the golangci-lint-action version in CI.
KUSTOMIZE_VERSION ?= v5.8.1
CONTROLLER_TOOLS_VERSION ?= v0.22.0
GOLANGCI_LINT_VERSION ?= v2.15.0

include ../../Makefile.common
