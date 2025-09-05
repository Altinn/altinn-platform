SHELL := bash

APP_NAME        ?= lakmus
IMAGE           ?= $(APP_NAME):local
CMD_DIR         ?= ./cmd

METRICS_PORT    ?= 8080
METRICS_ADDR    ?= :$(METRICS_PORT)
TICK_INTERVAL   ?= 60s
SUBSCRIPTION_ID ?= fake-sub
INTEG_WAIT_S    ?= 5

GO ?= go
CONTAINER_RUNTIME ?= podman   # override with CONTAINER_RUNTIME=docker

.PHONY: all test test-integration build clean docker-build docker-run fmt lint tidy manifests

all: fmt lint tidy test test-integration

ROOT_DIR_REPO := $(shell git rev-parse --show-toplevel)
PROJECT_DIR   ?= $(ROOT_DIR_REPO)/services/lakmus
MANIFESTS_DIR := $(PROJECT_DIR)/manifests

APP_OUT       := $(PROJECT_DIR)/config
FLUX_OUT      := $(ROOT_DIR_REPO)/flux/lakmus

## Generate Kubernetes and Flux manifests
manifests:
	@echo ">> Synthesizing app manifests into $(APP_OUT)"
	cd $(MANIFESTS_DIR) && npx cdk8s synth \
	  --app "npx ts-node main.ts" \
	  --output "$(APP_OUT)"

	@echo ">> Synthesizing flux manifests into $(FLUX_OUT)"
	cd $(MANIFESTS_DIR) && npx cdk8s synth \
	  --app "npx ts-node flux.ts" \
	  --output "$(FLUX_OUT)"

## Unit tests
test:
	$(GO) test -v ./...

## Build the container image
docker-build:
	$(CONTAINER_RUNTIME) build -t $(IMAGE) .

## Integration test: run container with fakes, curl metrics, stop container
test-integration: docker-build
	@set -euo pipefail; \
	CID=$$( $(CONTAINER_RUNTIME) run -d --rm \
		-p $(METRICS_PORT):$(METRICS_PORT) \
		-e AZURE_SUBSCRIPTION_ID=$(SUBSCRIPTION_ID) \
		$(IMAGE) \
			--subscription-id=$(SUBSCRIPTION_ID) \
			--tick-interval=$(TICK_INTERVAL) \
			--metrics-address=$(METRICS_ADDR) \
			--use-fakes ); \
	echo "Started container $$CID; waiting $(INTEG_WAIT_S)s..."; \
	sleep $(INTEG_WAIT_S); \
	if curl -sf "http://127.0.0.1:$(METRICS_PORT)/metrics" | grep -q 'lakmus_keyvault_secret_expiration_timestamp_seconds'; then \
		echo "✅ Metric found"; \
		$(CONTAINER_RUNTIME) rm -f "$$CID" >/dev/null; \
	else \
		echo "❌ Metric not found; recent logs:"; \
		$(CONTAINER_RUNTIME) logs --tail=200 "$$CID" || true; \
		$(CONTAINER_RUNTIME) rm -f "$$CID" >/dev/null; \
		exit 1; \
	fi

## Local build (binary only)
build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o bin/$(APP_NAME) $(CMD_DIR)

clean:
	rm -rf bin

## Run container in the foreground (Ctrl-C to stop)
docker-run: docker-build
	$(CONTAINER_RUNTIME) run --rm -p $(METRICS_PORT):$(METRICS_PORT) \
	  -e AZURE_SUBSCRIPTION_ID=$(SUBSCRIPTION_ID) \
	  $(IMAGE) \
	  --subscription-id=$(SUBSCRIPTION_ID) \
	  --tick-interval=$(TICK_INTERVAL) \
	  --metrics-address=$(METRICS_ADDR) \
	  --use-fakes

fmt:
	$(GO) fmt ./...

lint:
	golangci-lint run

tidy:
	${GO} mod tidy
