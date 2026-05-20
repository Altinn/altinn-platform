# Add dis-cache-operator (RFC 0014) — self-service Azure Managed Redis

This PR scaffolds a new operator, `dis-cache-operator`, that reconciles a `Redis` CR into Azure Managed Redis (`Microsoft.Cache/redisEnterprise`) via Azure Service Operator. It mirrors the proven patterns from `dis-vault-operator` (single-resource-per-CR, federated-identity-owned) and `dis-pgsql-operator` (private endpoint + shared private DNS + AKS VNet link).

See [RFC 0014](rfcs/0014-self-service-managed-redis.md) for the full design.

> The operator was initially scaffolded under the name `dis-redis-operator`; it was renamed to `dis-cache-operator` in this branch so the naming can later cover Valkey and other managed cache backends without another rename.

## Feature Behavior (BDD)

**Given** a `Redis` custom resource in the team namespace that references a ready `ApplicationIdentity` via `spec.identityRef`,
**When** the operator reconciles the CR,
**Then** it computes a deterministic Azure cluster name from `namespace + name + environment`,
**And** it creates ASO `RedisEnterprise` (cluster) and `RedisEnterpriseDatabase` resources with `accessKeysAuthentication=Disabled`, TLS-only client protocol by default, and port 10000,
**And** it creates an ASO `PrivateEndpoint` targeting the cluster in the configured AKS data subnet,
**And** it get-or-creates the shared `privatelink.redis.azure.net` private DNS zone and the AKS VNet link to it (label-managed, not owner-referenced to any single CR),
**And** it publishes status conditions `IdentityReady`, `ClusterReady`, `DatabaseReady`, `PrivateEndpointReady`, `PrivateDNSReady`, `AccessPolicyReady`, and an aggregated `Ready`,
**And** it populates `status.azureName`, `status.hostName`, `status.port`, `status.clusterResourceId`, `status.databaseResourceId`, and `status.ownerPrincipalId`.

**Given** the referenced identity is not yet ready (missing `status.principalId` or `Ready=True`),
**When** the operator reconciles,
**Then** it sets `IdentityReady=False` with reason `IdentityNotReady`, emits dependent ASO conditions as `IdentityNotReady` (rather than reading stale state), leaves Azure resources untouched, and requeues after 5 seconds.

**Given** the `Redis` CR specifies `spec.serviceAccountRef` instead of `spec.identityRef`,
**When** the operator reconciles,
**Then** it resolves the principal from the workload-identity annotations (`azure.workload.identity/client-id` and `dis.altinn.cloud/principal-id`) on the referenced `ServiceAccount`.

**Given** the `Redis` CR is deleted,
**When** the operator observes the deletion,
**Then** Kubernetes garbage collection cascades deletion to the owner-referenced cluster, database, and private endpoint resources, while the shared DNS zone and VNet link remain (they outlive any single CR).

**Given** the `PrivateDnsZonesVirtualNetworkLink` is not yet ready,
**When** the operator computes `PrivateDNSReady`,
**Then** it requires both the zone and the VNet link to be Ready before reporting True, so applications never see a True DNS condition while name resolution from AKS still fails.

**Given** any of `ClusterReady`, `DatabaseReady`, `PrivateEndpointReady`, or `PrivateDNSReady` is not yet True,
**When** the reconcile loop completes,
**Then** the controller requeues after `provisioningRequeueDelay` instead of waiting only for an owned-resource watch event (the shared DNS zone is label-managed and never fires watches).

> Note: `AccessPolicyAssignment` reconciliation is deferred to a follow-up PR (the upstream ASO type is not yet available in v2.17.0). The `AccessPolicyReady` condition reports `Unknown` / `Pending` until then.

## ASCII Diagram

```
                       ┌──────────────────────────────┐
                       │ Team namespace               │
                       │                              │
                       │  Redis CR ──ref──> AppIdent. │
                       │     │                  │     │
                       │     │ (controller      │     │
                       │     │  resolves        │     │
                       │     │  principalId)    │     │
                       └─────┼──────────────────┼─────┘
                             │                  │
                             ▼                  │
            ┌────────────────────────────┐      │
            │ dis-cache-operator         │      │
            └─────┬───────────┬──────────┘      │
                  │           │                 │
       owns: ┌────▼───┐  ┌────▼──────┐          │
             │ ASO    │  │ ASO       │          │
             │ Cluster│  │ Database  │          │
             └────┬───┘  └─────┬─────┘          │
                  │            │                │
        ┌─────────▼──┐    ┌────▼─────────┐      │
        │ ASO        │    │ (future PR)  │      │
        │ Private    │    │ Access       │      │
        │ Endpoint   │    │ Policy       │      │
        └─────┬──────┘    │ Assignment   │      │
              │           └──────────────┘      │
              │                                 │
              ▼                                 │
   ┌─────────────────────────────────────┐      │
   │ shared (label-managed, namespace-   │      │
   │ scoped, not owner-ref'd to any CR): │      │
   │ - PrivateDnsZone:                   │      │
   │     privatelink.redis.azure.net     │      │
   │ - PrivateDnsZonesVirtualNetworkLink │      │
   │     → AKS VNet                      │      │
   └─────────────────┬───────────────────┘      │
                     │                          │
                     ▼                          │
          ┌──────────────────────┐              │
          │ Azure subscription   │              │
          │ ┌────────────────┐   │   ┌──────────▼───────────┐
          │ │ RedisEnterprise│◀──┼───│ Workload pod         │
          │ │ + Database     │   │   │ (TLS 10000, Entra    │
          │ │ + Priv. EP     │   │   │  token via federated │
          │ └────────────────┘   │   │  workload identity)  │
          └──────────────────────┘   └──────────────────────┘
```

## Test plan

- [ ] `cd services/dis-cache-operator && make fmt vet test lint` — all green locally
- [ ] `make manifests` — CRD reflects the `RedisPersistence` `aof`/`rdb` mutual-exclusion XValidation rule
- [ ] CI: `dis-cache-lint-test.yml` runs golangci-lint and `make test` on the new path filter
- [ ] CI: `dis-cache-release.yml` builds the image on merge to main; release-please opens a `dis-cache-v0.1.0` PR
- [ ] Apply `config/samples/redis_v1alpha1_redis.yaml` against a Kind cluster; verify the CR is admitted and conditions surface as expected
