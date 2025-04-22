#!/usr/bin/env bash
set -euo pipefail

rm -rf .build .dist .conf
mkdir -p .build .dist .conf

generate-k6-manifests
