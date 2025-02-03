#!/usr/bin/env bash
#set -euxo pipefail
set -euo pipefail

# Create a dist folder where the manifests for deployment will be created
mkdir -p .dist
DIST_FOLDER="$(pwd)/.dist"


echo "filepath: ${INPUT_TEST_SCRIPT_FILEPATH}"
echo "namespace: ${INPUT_NAMESPACE}"


# Get the path to the directory where the script file is located
TEST_SCRIPT_DIR=$(dirname $(realpath "$INPUT_TEST_SCRIPT_FILEPATH"))

NAME=$(cat "$TEST_SCRIPT_DIR/conf.yaml" | yq '.test_run.name')

build_dir=$(mktemp -d)
cd "${build_dir}"

# Create the archive.tar file
k6 archive /github/workspace/"${INPUT_TEST_SCRIPT_FILEPATH}"

# Create a configmap from the archive
kubectl create configmap "${NAME}" \
        --from-file=archive.tar \
        -o json \
        -n "${INPUT_NAMESPACE}" \
        --dry-run=client \
        > "${DIST_FOLDER}"/configmap.json


jb init && \
jb install github.com/jsonnet-libs/k8s-libsonnet/1.32@main

# Generate the manifests based on the required inputs
jsonnet --jpath vendor \
        --ext-str userconfig="$(cat $TEST_SCRIPT_DIR/conf.yaml)" \
        --ext-str k6clusterconfig="$(cat /github/workspace/actions/k6/infra/k6_cluster_conf.yaml)" \
        --ext-str timestamp="$(date '+%Y%m%dT%H%M%S')" \
        --multi "${DIST_FOLDER}" /github/workspace/actions/k6/main.jsonnet

# Only for debug purposes
cat "${DIST_FOLDER}"/* | yq -p=json