#!/usr/bin/env sh
set -euo

rm -rf .build .dist .conf
mkdir -p .build .dist .conf

generate-k6-manifests
