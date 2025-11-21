# Deploy

```bash
kubectl apply -k flux/
```

# Test

```bash
kubectl create job --from=cronjob/altinn-uptime-sync test-$(date +%s) -n monitoring
kubectl logs -f job/test-<timestamp> -n monitoring
```

# Rollback

```bash
kubectl delete -k flux/
```
