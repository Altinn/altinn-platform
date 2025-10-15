# Grafana Redirect Configuration

This directory contains Flux configurations for redirecting Grafana monitoring traffic to Azure Managed Grafana.

## Overview

The configuration consists of two main components:

1. **Middleware** (`middleware.yaml`): A Traefik middleware that performs regex-based redirection
2. **IngressRoute** (`ingressroute.yaml`): A Traefik IngressRoute that catches `/monitor` requests and applies the redirect middleware

## How it works

When a user accesses `https://${K8S_DNS_NAME}/monitor/*`, the IngressRoute catches the request and applies the redirect middleware, which:

1. Matches requests with the pattern: `^https?://(.*)altinn\.(no|cloud)/monitor(.*)`
2. Redirects them permanently to: `${EXTERNAL_GRAFANA_URL}${captured_path}`

This effectively redirects all monitoring requests from any Altinn domain to the Azure Managed Grafana instance while preserving the original path.

## Required Variables

The following variables must be provided via Flux variable substitution:

| Variable | Description | Example |
|----------|-------------|---------|
| `EXTERNAL_GRAFANA_URL` | The full URL of the Azure Managed Grafana instance | `https://altinn-grafana-test-xyz.eno.grafana.azure.com` |
| `K8S_DNS_NAME` | The DNS name of the Kubernetes cluster/application | `platform.at22.altinn.cloud` |

## Deployment

This configuration is automatically deployed as part of the grafana-operator kustomization. The resources are created in the `traefik` namespace.

## Resources Created

- `Middleware/redirect-to-azure-grafana` in `traefik` namespace
- `IngressRoute/kube-prometheus-stack-grafana` in `traefik` namespace

## Original Terraform Equivalent

This Flux configuration replaces the following Terraform resources:

```hcl
resource "kubectl_manifest" "grafana_redirect_middleware" {
  yaml_body = <<YAML
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: redirect-to-azure-grafana
  namespace: traefik
spec:
  redirectRegex:
    permanent: true
    regex: ^https?://(.*)altinn\.(no|cloud)/monitor(.*)
    replacement: ${azurerm_dashboard_grafana.grafana.endpoint}$4
YAML
}

resource "kubectl_manifest" "grafana_redirect_ingressroute" {
  yaml_body = <<YAML
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: kube-prometheus-stack-grafana
  namespace: traefik
spec:
  entryPoints:
  - https
  routes:
  - kind: Rule
    match: Host(${var.appdnsname})&&PathPrefix(`/monitor`)
    middlewares:
    - name: redirect-to-azure-grafana
      namespace: traefik
    services:
    - kind: TraefikService
      name: noop@internal
YAML
}
```
