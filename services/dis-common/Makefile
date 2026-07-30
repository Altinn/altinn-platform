CACHE_DIR ?= $(CURDIR)/.cache
GOCACHE ?= $(CACHE_DIR)/go-build
GOLANGCI_LINT_CACHE ?= $(CACHE_DIR)/golangci-lint

LOCALBIN ?= $(CURDIR)/bin
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.12.2

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter.
	$(GOLANGCI_LINT) run

.PHONY: run-checks-ci
run-checks-ci: test lint ## Run all CI checks.

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: cache-setup
cache-setup: ## Create local build caches for sandboxed runs.
	@mkdir -p "$(GOCACHE)" "$(GOLANGCI_LINT_CACHE)"

CACHE_TARGETS := fmt-cache vet-cache test-cache lint-cache run-checks-ci-cache
$(CACHE_TARGETS): export GOCACHE := $(GOCACHE)
$(CACHE_TARGETS): export GOLANGCI_LINT_CACHE := $(GOLANGCI_LINT_CACHE)

.PHONY: $(CACHE_TARGETS)
fmt-cache: cache-setup ## fmt with repository-local caches.
	$(MAKE) fmt
vet-cache: cache-setup ## vet with repository-local caches.
	$(MAKE) vet
test-cache: cache-setup ## test with repository-local caches.
	$(MAKE) test
lint-cache: cache-setup ## lint with repository-local caches.
	$(MAKE) lint
run-checks-ci-cache: cache-setup ## Run all CI checks with repository-local caches.
	$(MAKE) run-checks-ci
