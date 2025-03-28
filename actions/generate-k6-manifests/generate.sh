#!/usr/bin/env bash
set -euo pipefail

rm -rf .build .dist .conf
mkdir -p .build .dist .conf

export GOCACHE=$(mktemp -d)
export GOMODCACHE=$(mktemp -d)

#cd actions/generate-k6-manifests/ && \
### go mod tidy && \
### go build -buildvcs=false . && \
#go build . && \
#cd ../../ && \
#mv actions/generate-k6-manifests/generate-k6-manifests . && \
/github/workspace/generate-k6-manifests

# rm ./generate-k6-manifests
