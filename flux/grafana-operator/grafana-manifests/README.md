# Grafana Manifests

This directory contains Grafana manifests organized into base and application-specific configurations.

## Structure

```
grafana-manifests/
├── README.md
├── base/
│   ├── kustomization.yaml
│   ├── folders.yaml
│   └── dashboards/
│       ├── kustomization.yaml
│       ├── altinn-blackbox-exporter.yaml
│       ├── altinn-publicip.yaml
│       ├── altinn-traefik-official.yaml
│       ├── fluxcd-*.yaml
│       └── linkerd-*.yaml
└── apps/
    ├── kustomization.yaml
    └── dashboards/
        ├── kustomization.yaml
        └── altinn-pod-console-error-logs.yaml
```

## Base Configuration

The `base/` directory contains shared Grafana manifests that are common across environments:

- **Folders**: Grafana folder organization (`folders.yaml`)
- **Infrastructure Dashboards**: Core monitoring dashboards for:
  - Altinn platform (blackbox-exporter, publicip, traefik)
  - FluxCD (cluster stats, control plane, deployments)
  - Linkerd service mesh (daemonset, deployment)

## Application Configuration

The `apps/` directory contains apps cluster specific Grafana manifests:

- **Dashboards**: Apps cluster specific monitoring dashboards
- **Future**: Ready for expansion with alerts, datasources, etc.

The apps configuration includes the base configuration via `../base` reference, so deploying apps will include both base and apps cluster specific manifests.

## Usage

### Deploy Base Only
Point Flux Kustomization to:
```
oci://your-registry/grafana-operator/grafana-manifests/base
```

### Deploy Apps (includes Base)
Point Flux Kustomization to:
```
oci://your-registry/grafana-operator/grafana-manifests/apps
```

## Adding New Manifests

### New Base Dashboard
1. Add the dashboard YAML file to `base/dashboards/`
2. Update `base/dashboards/kustomization.yaml` to include the new file

### New App Dashboard
1. Add the dashboard YAML file to `apps/dashboards/`
2. Update `apps/dashboards/kustomization.yaml` to include the new file

### New Manifest Type (e.g., Alerts)
1. Create new directory: `apps/alerts/`
2. Add `apps/alerts/kustomization.yaml`
3. Add alert YAML files to the alerts directory
4. Update `apps/kustomization.yaml` to include `- alerts`

## Manifest Sources

Dashboards are sourced from the [Altinn Grafana Dashboards repository](https://github.com/Altinn/altinn-dashboards-grafana) using the `${RELEASE_BRANCH}` variable for version control.

## Dependencies

These manifests depend on:
- Grafana Operator being deployed
- External Grafana instance configured via `EXTERNAL_GRAFANA_URL`
- Proper RBAC permissions for the Grafana Operator