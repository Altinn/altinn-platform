#!/usr/bin/env bash
set -euo pipefail
KUBECTL_VERSION="$(curl -L -s https://dl.k8s.io/release/stable.txt)"
KUBESEAL_VERSION=$(curl -s https://api.github.com/repos/bitnami-labs/sealed-secrets/tags | jq -r '.[0].name')
JSONNET_VERSION=$(curl -s https://api.github.com/repos/google/jsonnet/tags | jq -r '.[0].name')
K6_VERSION=$(curl -s https://api.github.com/repos/grafana/k6/tags | jq -r '.[0].name')
JB_VERSION=$(curl -s https://api.github.com/repos/jsonnet-bundler/jsonnet-bundler/tags | jq -r '.[0].name')

echo "**** Latest versions ****"
echo "KUBECTL:  ${KUBECTL_VERSION}"
echo "KUBESEAL: ${KUBESEAL_VERSION}"
echo "JSONNET:  ${JSONNET_VERSION}"
echo "K6:       ${K6_VERSION}"
echo "JB:       ${JB_VERSION}"
echo "*************************"