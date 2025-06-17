# Traefik Configuration

This directory contains the Flux configuration for deploying Traefik as the ingress controller in the Altinn platform.

## Overview

Traefik is configured as a reverse proxy and load balancer that handles incoming traffic routing for the Altinn platform. This setup uses Flux CD for GitOps-based deployment and management.

## Components

### Files Structure

- `namespace.yaml` - Creates the `traefik` namespace
- `helmrepository.yaml` - Defines the Traefik Helm repository source
- `helmrelease.yaml` - Contains the Traefik deployment configuration
- `kustomization.yaml` - Flux kustomization resource that manages all components

### Helm Releases

#### 1. Traefik CRDs (`traefik-crds`)
- **Chart Version**: 1.8.1
- **Purpose**: Installs Custom Resource Definitions required by Traefik
- **Reconciliation**: Every 1 hour with 5 retry attempts

#### 2. Altinn Traefik (`altinn-traefik`)
- **Chart Version**: 36.1.0
- **Purpose**: Main Traefik deployment
- **Dependencies**: Requires `traefik-crds` to be installed first
- **Registry**: Uses Altinn's Azure Container Registry (`altinncr.azurecr.io`)

## Key Configuration Features

### High Availability
- **Replicas**: 3 instances for redundancy
- **Pod Disruption Budget**: Ensures minimum 1 pod available during updates
- **Anti-Affinity**: Pods prefer to run on different nodes

### Network Configuration
- **Dual Stack**: Supports both IPv4 and IPv6
- **External Traffic Policy**: Configurable (defaults to Local)
- **Load Balancer**: Azure Load Balancer with dedicated public IPs

### Security
- **TLS Configuration**: 
  - Minimum TLS 1.2
  - Strong cipher suites (AES-GCM and AES-CBC)
  - HSTS headers with 2-year max-age
- **Trusted IPs**: Configured for AKS system and worker pool IP ranges
- **HTTP to HTTPS Redirect**: Automatic permanent redirect

### Service Mesh Integration
- **Linkerd**: Service mesh injection enabled
- **Skip Ports**: Bypasses mesh for ports 8000 and 8443
- **Resource Limits**: Optimized proxy resource allocation

### Monitoring
- **Prometheus Metrics**: Enabled with ServiceMonitor
- **Custom Relabeling**: Node name labeling for better observability
- **Metric Filtering**: Filters out specific fluentd metrics

## Port Configuration

| Port | Purpose | External Port | Protocol | Features |
|------|---------|---------------|----------|----------|
| 8000 | HTTP | 80 | TCP | Redirects to HTTPS |
| 8443 | HTTPS | 443 | TCP | TLS enabled |

## Middleware Configuration

### HSTS Header Middleware
Applied to both `traefik` and `default` namespaces:
- **Include Subdomains**: Yes
- **Max Age**: 2 years (63,072,000 seconds)
- **Preload**: Enabled

### Root Ingress Route
- Catches all traffic with `PathPrefix(/)`
- Applies HSTS headers
- Routes to internal noop service (default handler)

## Environment Variables

The configuration uses several environment variables that must be set:

- `AKS_SYSP00L_IP_PREFIX_0/1` - AKS system pool IP prefixes
- `AKS_WORKPOOL_IP_PREFIX_0/1` - AKS worker pool IP prefixes
- `AKS_NODE_RG` - Azure resource group for AKS nodes
- `PUBLIC_IP_V4` - Public IPv4 address
- `PUBLIC_IP_V6` - Public IPv6 address
- `EXTERNAL_TRAFFIC_POLICY` - External traffic policy (optional, defaults to Local)

## Resource Requirements

### Traefik Pods
- **CPU Request**: 100m
- **Memory Request**: 50Mi

### Linkerd Proxy (per pod)
- **CPU Request**: 50m
- **Memory Request**: 20Mi
- **Memory Limit**: 250Mi

## Deployment

This configuration is managed by Flux CD. Changes to the configuration files will be automatically applied to the cluster according to the reconciliation schedule (every 1 hour).

### Manual Reconciliation
To force immediate reconciliation:
```bash
flux reconcile helmrelease altinn-traefik -n traefik
```

## Troubleshooting

### Common Issues
1. **CRD Installation**: Ensure `traefik-crds` is installed before `altinn-traefik`
2. **Environment Variables**: Verify all required environment variables are set
3. **SSL Certificate**: Ensure `ssl-cert` secret exists in the namespace
4. **Load Balancer**: Check Azure Load Balancer configuration and IP assignments

### Monitoring
- Check Traefik dashboard (if enabled)
- Monitor Prometheus metrics
- Review pod logs: `kubectl logs -n traefik -l app.kubernetes.io/name=traefik`

## Security Considerations

- TLS certificates are managed through the `ssl-cert` secret
- Trusted IP ranges should be regularly reviewed and updated
- HSTS preload is enabled for enhanced security
- Service mesh provides additional security layers