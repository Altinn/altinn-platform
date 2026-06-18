# dis-console is a plain net/http Go service (not a kubebuilder operator). It
# reuses the shared ../../Makefile.common for the generic Go tasks (fmt, vet,
# lint, build, docker, tooling, cache, checks) and overrides the build/test
# wiring so none of the kubebuilder codegen/envtest machinery runs.

APP_NAME ?= dis-console
IMG ?= $(APP_NAME):local
CONTAINER_TOOL ?= podman

# dis-console is a self-contained module with no sibling dependencies. The
# operators' `make workspace` may leave a root go.work that does not list this
# service, which would break `go` commands here; ignore any ambient workspace.
# No-op in CI (no go.work present).
export GOWORK := off

# Plain-service build/test wiring (no controller-gen / envtest).
BINARY := $(APP_NAME)
MAIN := cmd/main.go
BUILD_DEPS := fmt vet
TEST_DEPS := fmt vet
GO_TEST_CMD := go test ./... -coverprofile cover.out
RUN_ARGS := agent --local

include ../../Makefile.common

##@ E2E (Kind)

# Store e2e: stand up a trust-auth Postgres on Kind, port-forward it, and run
# the e2e-tagged store test against it over pgx. Mirrors the operators' Kind CI
# job but without the controller/ginkgo ceremony (dis-console is a plain service).
E2E_TEST_TIMEOUT ?= 10m
E2E_DB_PORT ?= 55432
E2E_DB_URI := postgres://postgres@localhost:$(E2E_DB_PORT)/postgres?sslmode=disable

.PHONY: test-e2e-kind-ci
test-e2e-kind-ci: cleanup-test-e2e setup-test-e2e ## Run the store e2e against a trust-auth Postgres on Kind, then tear down.
	@set -e; \
	pf=""; \
	trap 'if [ -n "$$pf" ]; then kill "$$pf" 2>/dev/null || true; fi; $(MAKE) cleanup-test-e2e' EXIT; \
	$(KUBECTL) apply -f config/kind/postgres.yaml; \
	$(KUBECTL) -n default rollout status deploy/postgres --timeout=180s; \
	$(KUBECTL) -n default port-forward svc/postgres $(E2E_DB_PORT):5432 >/dev/null 2>&1 & \
	pf=$$!; \
	DISCONSOLE_TEST_DB_URI="$(E2E_DB_URI)" go test -tags=e2e ./test/e2e/... -v -timeout=$(E2E_TEST_TIMEOUT)
