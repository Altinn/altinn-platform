#!/bin/bash

# Set the provider to Podman
export KIND_EXPERIMENTAL_PROVIDER=podman

CLUSTER_NAME=test-obs-operator
APP_NAME=obs-operator

# Create kind cluster
kind create cluster --name ${CLUSTER_NAME} --image kindest/node:v1.23.0

# Install Prometheus Operator CRDs
kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml

# Apply PrometheusRuleGroup CRD
kubectl apply -f config/crd/bases/alertsmanagement.azure.com_prometheusrulegroups.yaml

# Build operator image
podman build -t ${APP_NAME}:latest .

# Load image into kind
podman save -o ${APP_NAME}.tar ${APP_NAME}:latest
kind load image-archive --name ${CLUSTER_NAME} ${APP_NAME}.tar

# Deploy operator
make install
make deploy

# Apply sample PrometheusRule
kubectl apply -f sample-prometheusrule.yaml
