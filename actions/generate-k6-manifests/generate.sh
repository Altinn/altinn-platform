#!/usr/bin/env bash
set -euo pipefail

rm -rf .build .dist .conf

mkdir -p .build
mkdir -p .dist
mkdir -p .conf

cd .build
jb init && \
jb install github.com/jsonnet-libs/k8s-libsonnet/1.32@main
cd ..

export GOCACHE=$(mktemp -d)
export GOMODCACHE=$(mktemp -d)

cd actions/generate-k6-manifests/ && \
# go mod tidy && \
# go build -buildvcs=false . && \
go build . && \
cd ../../ && \
mv actions/generate-k6-manifests/generate-k6-manifests . && \
./generate-k6-manifests

rm ./generate-k6-manifests

# Debug purposes only
# cat .dist/*.json | yq -p=json
# ls -l .dist
# cat .dist/*.json | yq -p=json > ".dist/deploy.yaml"

# rm -rf .dist/*.json .build .conf
# rm -rf .build
# rm -rf .build/vendor .build/archive.tar .build/jsonnetfile.json .build/jsonnetfile.lock.json .dist/configmap-* .dist/slo-* .dist/testrun-*
rm -rf .build/vendor .build/archive.tar .build/jsonnetfile.json .build/jsonnetfile.lock.json
