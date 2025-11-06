# Azure Monitor Container Agent Configuration

This directory contains the ConfigMap for configuring the Azure Monitor container agent in the AKS cluster.

## Configuration

- `container-azm-ms-agentconfig.yaml`: ConfigMap that controls log collection settings for stdout, stderr, environment variables, and Kubernetes events
- `kustomization.yaml`: Kustomize configuration for deploying the ConfigMap

## Settings

The agent is configured to:
- Disable stdout log collection
- Enable stderr log collection (excluding kube-system and monitoring namespaces)
- Disable environment variable collection
- Disable container log enrichment
- Disable collection of all Kubernetes events (only abnormal events are collected)
