.DEFAULT_GOAL = all

BINARY = dis
PROJECT_NAME = github.com/altinn/altinn-platform/disctl
VERSION_PKG = github.com/altinn/altinn-platform/disctl/internal/version
CONCURRENCY := 4
GO := go
TAGS ?= ""
COMMIT := $(shell git rev-parse HEAD)
VERSION := "dev"
DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')


# Check if windows
ifeq ($(OS),Windows_NT)
	BINARY := $(BINARY).exe
endif

all: test build
build:
	$(GO) build -o $(BINARY) -ldflags "-X $(VERSION_PKG).version=$(VERSION) -X $(VERSION_PKG).commit=$(COMMIT)  -X $(VERSION_PKG).date=$(DATE) -s -w" -v

tidy:
	${GO} mod tidy

deps:
	$(GO) get -tags ${TAGS} -t ./...
	make tidy

test: build
	${GO} test --timeout 30m -short -count 1 -parallel ${CONCURRENCY} ./...

coverage: build
	${GO} test --timeout 30m -count 1 -coverpkg=${PROJECT_NAME}/... -race -coverprofile=coverage.out -parallel ${CONCURRENCY} ./...
	${GO} tool cover -html=coverage.out -o=coverage-report.html
	printf "Coverage report available at coverage-report.html\n"

clean:
	$(GO) clean
	rm -f $(BINARY)
	rm -f coverage*
	rm -rf dist

format:
	$(GO) fmt ./...

lint:
	golangci-lint run

snapshot:
	goreleaser build --snapshot --clean	

.PHONY: all build test
