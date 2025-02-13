#!/usr/bin/env bash
#set -euxo pipefail
set -euo pipefail

# Create a dist folder where the manifests for deployment will be created
mkdir -p .dist
DIST_FOLDER="$(pwd)/.dist"


echo "filepath: ${INPUT_TEST_SCRIPT_FILEPATH}"
echo "namespace: ${INPUT_NAMESPACE}"
echo "extra CLI args to k6 archive: ${INPUT_COMMAND_LINE_ARGS}"

# Get the path to the directory where the script file is located
TEST_SCRIPT_DIR=$(dirname $(realpath "$INPUT_TEST_SCRIPT_FILEPATH"))

NAME=$(cat "$TEST_SCRIPT_DIR/conf.yaml" | yq '.test_run.name')

build_dir=$(mktemp -d)
cd "${build_dir}"

# Create the archive.tar file
k6 archive --config "${TEST_SCRIPT_DIR}/config.json" /github/workspace/"${INPUT_TEST_SCRIPT_FILEPATH}"

# Debug purposes only
tar -xvf archive.tar > /dev/null
cat metadata.json

# Create a configmap from the archive
kubectl create configmap "${NAME}-${INPUT_SUFFIX}" \
        --from-file=archive.tar \
        -o json \
        -n "${INPUT_NAMESPACE}" \
        --dry-run=client \
        > "${DIST_FOLDER}"/configmap.json


jb init && \
jb install github.com/jsonnet-libs/k8s-libsonnet/1.32@main

mkdir temp_dist

# Generate the manifests based on the required inputs
jsonnet --jpath vendor \
        --ext-str userconfig="$(cat $TEST_SCRIPT_DIR/conf.yaml)" \
        --ext-str testscriptdir="$TEST_SCRIPT_DIR" \
        --ext-str extra_cli_args="$INPUT_COMMAND_LINE_ARGS" \
        --ext-str k6clusterconfig="$(cat /github/workspace/actions/generate-k6-manifests/infra/k6_cluster_conf.yaml)" \
        --ext-str suffix="$INPUT_SUFFIX" \
        --multi temp_dist /github/workspace/actions/generate-k6-manifests/main.jsonnet

# Quote the values since the TestRun validation is quite strict
cat temp_dist/*.json | yq -p=json

cat temp_dist/*.json | yq -p=json > "${DIST_FOLDER}/deploy.yaml"
