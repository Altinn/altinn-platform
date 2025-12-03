# Altinn Uptime Monitoring

Auto-generates ServiceMonitors for Altinn organizations using Prometheus Blackbox Exporter.

## Configuration

- **Extra targets**: Edit `configmaps/extra-targets.yaml`
- **Maintenance**: Edit `configmaps/maintenance-targets.yaml`
- **Script**: Edit `scripts/generate_targets.sh`

## Manual Run

```bash
kubectl create job --from=cronjob/altinn-uptime-sync manual-$(date +%s) -n monitoring
```
