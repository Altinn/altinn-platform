# dis-redis-operator

Self-service provisioning of Azure Managed Redis (`Microsoft.Cache/redisEnterprise`) for DIS application teams. App teams declare a `Redis` custom resource in their namespace, and the operator reconciles it into an Azure Redis Enterprise cluster + database with a private endpoint and Entra-only data-plane access.

See [RFC 0014 - Self-service Managed Redis](../../rfcs/0014-self-service-managed-redis.md) for the full design.

## Quick start

- `make help` — list all targets.
- `make run-checks-ci-cache` — required pre-PR verification (fmt, generate, manifests, test, lint).
- `make install-cache && kubectl apply -f config/samples/` — install CRDs and apply a sample.

See `AGENTS.md` for contribution conventions.
