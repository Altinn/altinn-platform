# Grafana Operator

## Description

This directory contains the Flux configuration for deploying the Grafana Operator using Helm and configuring an external Grafana instance.

## Deployment Image

The deployment uses the following image:
- **Repository**: `altinncr.azurecr.io/ghcr.io/grafana/grafana-operator`
- **Original Source**: `ghcr.io/grafana/grafana-operator`

The image is pulled from Altinn's Azure Container Registry (altinncr.azurecr.io) which mirrors the official Grafana Operator image from GitHub Container Registry.

## Configuration

### Required Environment Variables

The following environment variables must be set for proper deployment:

| Variable | Description | Example |
|----------|-------------|---------|
| `EXTERNAL_GRAFANA_URL` | URL of the external Grafana instance | `https://grafana.example.com` |
| `GRAFANA_ADMIN_APIKEY` | Admin API key for Grafana authentication | `glsa_xxxxxxxxxxxxxxxxxxxx` |

### Components

1. **Namespace**: Creates `grafana` namespace with Linkerd injection enabled
2. **Helm Repository**: Configures Grafana Helm repository source
3. **Helm Release**: Deploys the Grafana Operator with custom configuration
4. **External Grafana**: Configures connection to external Grafana instance
5. **API Key Secret**: Stores Grafana admin API key securely

### Features Enabled

- **Service Monitor**: Prometheus monitoring enabled with custom label limits
- **Dashboard Management**: Automatic dashboard provisioning enabled
- **Custom Patches**: ServiceMonitor patches for Azure Monitor compatibility

### Resource Customizations

The deployment includes post-render patches that:
- Update ServiceMonitor API version to `azmonitoring.coreos.com/v1`
- Set label limits: 63 labels max, 511 char name limit, 1023 char value limit
- Enable CRD management with create/replace strategy

### Security

- API keys are stored as Kubernetes secrets
- External Grafana connection uses secure API key authentication
- Namespace has Linkerd service mesh injection enabled

## Usage

Ensure environment variables are properly substituted in your Flux configuration before deployment.
