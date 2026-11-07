- Feature Name: `central_observability`
- Title: Central observability platform for logs and traces
- Start Date: 2026-11-07
- RFC PR: [altinn/altinn-platform#0000](https://github.com/altinn/altinn-platform/pull/0000)
- Github Issue: [altinn/altinn-platform#0000](https://github.com/altinn/altinn-platform/issues/0000)
- Product/Category: Observability
- State: **REVIEW** (possible states are: **REVIEW**, **ACCEPTED** and **REJECTED**)

# Summary
[summary]: #summary

We replace Azure Application Insights as the store for OpenTelemetry (OTel) traces and application logs.
The design uses ClickHouse as the storage for traces and logs. All other parts of the design depend on this choice.
A central monitoring cluster on Azure Kubernetes Service (AKS) runs ClickHouse, OTel gateway collectors and Grafana.
Workload clusters send OTel Protocol (OTLP) data to the gateways and authenticate with Microsoft Entra ID workload identity.
Senders that cannot use Entra ID authenticate with basic authentication.
The gateways set the tenant and the environment from the verified identity.
ClickHouse stores the data in shared tables and isolates tenants with row policies.
Grafana is the primary user interface (UI), with one Grafana instance for each tenant.
The Grafana Operator manages the Grafana instances and their resources.
Grafana alert rules on ClickHouse queries give alerts on logs and traces.
A safe access model for AI agents has high priority. Agents get read-only access for one tenant, through Grafana or through a restricted ClickHouse user.
Security logs for the Security Operations Centre (SOC) continue to go to Microsoft Sentinel.
Metrics stay in Azure Monitor managed service for Prometheus.

# Motivation
[motivation]: #motivation

## Reasons for the change

There are three reasons for the change, in order of priority.

### 1. Cost

- Log Analytics ingestion costs approximately 2.30–2.76 USD for each GB at list price, pay-as-you-go.
  The 100 GB/day commitment tier costs approximately 1.96 USD for each GB.
- Our own platform sends approximately 100 GiB/day from production.
  One test environment sends a comparable volume. Four smaller environments send less.
- Our own platform pays an estimated 15,000–17,500 USD/month at list price. Refer to [Cost estimate](#cost-estimate).
- We operate infrastructure for approximately 75 customers. Each customer has its own Application Insights workspace.
  Small workspaces cannot use commitment tiers, so each customer pays the pay-as-you-go price.
- If Application Insights writes to a workspace that has Sentinel enabled, the workspace also pays the Sentinel analysis charge.
  This charge also applies to traces that the SOC does not use.
- The cost grows linearly with the volume. Retention longer than 90 days adds a cost for each GB and month.

### 2. Portability

- Application Insights, Log Analytics and the Kusto Query Language (KQL) exist only in Azure.
  Our queries, dashboards and alert rules do not move to another platform.
- The proposed stack uses only open components: OTel, ClickHouse and Grafana. It runs on any Kubernetes cluster.
- The data stays in an open, OTel-based schema. We can export it, or connect another tool to it, without a conversion.
- The same stack works for our own platform and for all customers. Teams learn one set of tools.
- A customer can leave our platform and keep the same tools.

### 3. Limited OTel support in Application Insights

- Application Insights converts OTel data to its own data model (requests, dependencies, traces and exceptions).
  OTel attributes go into the generic `customDimensions` field.
- Queries on OTel attributes and on OTel semantic conventions are difficult to write and slow to run.
- The trace UI and the query model do not follow OTel concepts. The team does not like to query or operate it.
- New OTel features arrive in Application Insights later than in OTel-native tools.

### High priority requirement: a safe access model for AI agents

We want AI agents to investigate incidents with logs and traces. Today we cannot give AI agents safe access.
The design must give a safe access model for AI agents. This requirement has high priority.

The access model must have these properties:

| Property | Requirement |
|---|---|
| Read-only | An agent cannot change or delete data. The database enforces this, not only the agent tool. |
| One tenant | An agent reads the data of one tenant only. Row policies in the database enforce this. |
| Limits | Each query has limits for run time, memory, rows read and rows returned. Each agent has a quota for each hour. |
| No credentials in the agent | The default path goes through Grafana. The agent does not see database credentials. |
| Sensitive data | The gateway removes sensitive data before storage. Agents read masked views only. |
| Audit | The database logs each query with the user name of the agent. |

Refer to [AI agents](#ai-agents) for the design.

### Other problems

- Each customer also has its own managed Prometheus and Azure Managed Grafana. The tools are fragmented.

## Why the design uses ClickHouse

The design uses ClickHouse as the storage. ClickHouse is an open-source column-oriented database (Apache 2.0 licence).
These properties of ClickHouse decide the rest of the design:

| Property | Benefit for this design |
|---|---|
| Column storage with compression | Trace and log data compresses 6–10 times. Disk cost stays low at 100 GiB/day or more. |
| SQL | Developers and operators can run heavy analytical queries, for example p99 latency for each route over 30 days. |
| Grafana support | The official Grafana ClickHouse plugin shows traces and logs, and includes OTel dashboards and alert rules. |
| OTel support | The OTel Collector `clickhouse` exporter writes traces and logs directly. There is no proprietary agent. |
| Access control for each user | Row policies, settings profiles and quotas isolate tenants and limit AI agents in the database. |
| Data tiers | ClickHouse can move old data to Azure Blob Storage for long retention at a low cost. |
| Wide use for observability | ClickStack, SigNoz and Jaeger v2 use ClickHouse as the store for OTel data. |

[Rationale and alternatives](#rationale-and-alternatives) compares ClickHouse with the other options.

## Use cases

1. A developer opens Grafana, finds a slow request and opens the trace waterfall.
2. A developer jumps from a trace to the related logs, and from a log line to the trace.
3. An operator runs a heavy analytical query, for example p99 latency for each route over 30 days.
4. A customer sees only its own logs and traces in its own Grafana instance.
5. An AI agent investigates an incident with read-only, rate-limited access to one tenant.
6. The SOC continues to detect threats in Sentinel without new gaps.

## Expected outcome

- Cost: an estimated cost reduction of 50–65 % for logs and traces, including operations effort.
  Refer to [Cost estimate](#cost-estimate).
- Portability: one OTel-native stack of open components, operated with GitOps (Flux) and Terraform.
- OTel support: traces and logs keep the OTel data model. Grafana is the primary UI. SQL is available for heavy queries.
- AI agents: a safe access model for AI agents that the database enforces. This outcome has high priority.
- Hard tenant and environment isolation that the database enforces.

# Guide-level explanation
[guide-level-explanation]: #guide-level-explanation

## New concepts

| Concept | Meaning |
|---|---|
| Monitoring cluster | The central AKS cluster that runs ClickHouse, the ingest gateways and Grafana. |
| Edge collector | The OTel collector in a workload cluster. It already exists today. |
| Ingest gateway | An OTel collector in the monitoring cluster. It authenticates senders and writes to ClickHouse. |
| Tier | `prod` or `nonprod`. Each tier has its own ingest gateway, Entra application and ClickHouse cluster. |
| Tenant | A customer, or our own platform (`platform`). Each tenant has an identifier, for example `acme`. |
| Environment | An environment of a tenant, for example `at22` or `prod`. |
| Ingest role | An Entra app role with the value `ingest.<tenant>.<environment>`. It gives one sender access to one tenant and one environment. |
| Tenant Grafana instance | A Grafana instance for one tenant, managed by the Grafana Operator. It contains the data sources for that tenant only. |

## How a team sends data

Applications do not change. They continue to send OTLP to the edge collector in their cluster.
The edge collector gets a new exporter that sends to the ingest gateway of its tier.

```yaml
exporters:
  otlphttp/central:
    endpoint: https://otel-ingest-prod.monitoring.example.com
    compression: zstd
    auth:
      authenticator: azure_auth
    sending_queue:
      enabled: true
      storage: file_storage/queue
```

The edge collector gets an Entra token with workload identity. The cluster does not store a secret.
A sender that cannot use Entra ID uses basic authentication with a user name and a password.
The ingest gateway reads the ingest role from the token.
The gateway sets `tenant.id` and `deployment.environment.name` from the role.
The gateway replaces any value that the sender sets for these attributes.

## How a user reads data

1. The user signs in to Grafana with Entra ID.
2. The user opens the Grafana instance of the tenant, for example `https://acme.grafana.example.com`.
3. The user opens one of these views:
   - The "OpenTelemetry Traces Explorer" dashboard: service map and trace search.
   - The "OpenTelemetry Service Dashboard": rate, errors and duration (RED) for one service.
   - Explore: log search, the trace waterfall, and links between traces and logs.
   - The SQL editor: free SQL queries on ClickHouse.

A user can write any SQL in the Grafana instance.
ClickHouse row policies make sure that the user only gets rows of the tenant.

## How a team gets alerts

A team writes alert rules as Grafana alert rules on ClickHouse queries.
Examples are the error ratio of a service, the latency of a route and the count of error logs.
The rules are `GrafanaAlertRuleGroup` resources in Git. The Grafana Operator applies them to the Grafana instance of the tenant.
Notifications go to the contact points of the team, for example Slack or e-mail.

## How an AI agent reads data

There are two paths. Each path is read-only and limited to one tenant.

| Path | Use | Credential |
|---|---|---|
| `mcp-grafana` | Default. Log search, trace lookup, dashboards. | A `GrafanaServiceAccount` in the tenant Grafana instance. The agent does not see database credentials. |
| `mcp-clickhouse` | Deep SQL analysis. | A ClickHouse user with a read-only settings profile, a quota and access to masked views only. |

## How we add a customer

1. Add the tenant to the Terraform input `tenants`.
   Terraform creates the ingest roles, the identities, the federated credentials and the Entra group for Grafana sign-in.
2. Add the tenant to the inputs of the Grafana `ResourceSet`.
   The Grafana Operator creates the Grafana instance, the data sources, the dashboards and the alert rules.
3. Add the tenant to the ClickHouse access configuration in Git.
   Flux applies the role, the row policies and the users.
4. Add the new exporter to the edge collector in the customer cluster.

## How we remove a customer

1. Remove the ingest role assignments. The gateway rejects new data at once.
2. Delete the data with `DELETE FROM ... WHERE TenantId = '<tenant>'`, or let the time to live (TTL) remove it.
3. Remove the tenant from the Grafana `ResourceSet` inputs. Flux removes the Grafana instance.
4. Remove the ClickHouse users.

## What the SOC sees

Nothing changes for security logs that come from Azure platform paths.
Entra logs, Azure Activity, Defender, the AKS audit log and PostgreSQL audit logs continue to go to Sentinel.
Application security events go to Sentinel and to ClickHouse.
The SOC can read application traces and logs in the SOC Grafana instance.

## Migration for existing services

1. The edge collector sends to the ingest gateway and to Application Insights at the same time.
2. Teams compare the two views for two to four weeks.
3. We remove the Application Insights exporter for each environment when the team accepts the change.

# Reference-level explanation
[reference-level-explanation]: #reference-level-explanation

## Architecture

```
workload cluster (AKS)                              monitoring cluster (AKS)
┌──────────────────────────────┐                    ┌─────────────────────────────────────────┐
│ apps ─OTLP─▶ edge collector  │── OTLP/HTTP, TLS ─▶│ ingress ─▶ ingest gateway (prod)         │──▶ ClickHouse prod
│              azure_auth      │   Bearer token      │            oidc auth, tenant from role   │
└──────────────────────────────┘                    │ ingress ─▶ ingest gateway (nonprod)      │──▶ ClickHouse nonprod
                                                    │                                          │
                                                    │ Grafana OSS (one instance per tenant)    │◀── users, mcp-grafana
                                                    └─────────────────────────────────────────┘
Azure platform logs ── Diagnostic settings ──▶ Sentinel workspace (no change)
Prometheus metrics  ── Azure Monitor workspace (no change) ──▶ Grafana data source
```

## Storage: ClickHouse

### Clusters

There are two ClickHouse clusters, one for each tier.
The `nonprod` cluster contains the large test environment, so it gets a volume close to `prod`.
Separate clusters make sure that a load test does not slow down queries on `prod` data.

| | `prod` | `nonprod` |
|---|---|---|
| Topology | 1 shard, 2 replicas | 1 shard, 2 replicas |
| Replica size | 8 vCPU, 32 GiB RAM (for example `E8ds_v5`) | 4–8 vCPU, 16–32 GiB RAM |
| Disk for each replica | ~700 GiB Premium SSD v2 | ~450 GiB Premium SSD v2 |
| ClickHouse Keeper | 3 small pods | 3 small pods |
| Node pool | `ch-prod`, with a taint | `ch-nonprod`, with a taint |

The sizes are for our own platform only.
The volume from customers is not known yet. Refer to [Unresolved questions](#unresolved-questions).

### Sizing assumptions

- 100 GiB/day of raw data is approximately 1.2 MiB/s on average. Peaks are 3–4 times higher.
- Compression is 6–10 times. Measurements of the OTel exporter schema show a ratio of 5.9 ([Coroot](https://coroot.com/?p=1119)).
- Retention: `prod` 30 days, the large test environment 14 days, other environments 7 days.

| Tier | Raw GiB/day | On disk for each replica |
|---|---|---|
| `prod` | ~100 | 300–500 GiB |
| `nonprod` | ~160 | 180–305 GiB |

### Operator

We deploy ClickHouse with a Kubernetes operator through Flux.
The candidates are the [official ClickHouse operator](https://clickhouse.com/docs/products/kubernetes-operator/overview) and the Altinity operator.
The official operator uses ClickHouse Keeper and the `Replicated` database engine. Its API is `v1alpha1`.
The pilot decides which operator we use.

### Schema

We use the table layout of the OTel Collector `clickhouse` exporter (`otel_traces`, `otel_logs`).
The Grafana ClickHouse plugin and its OTel dashboards expect this layout.
We add two columns and change the sorting key:

```sql
TenantId LowCardinality(String) MATERIALIZED ResourceAttributes['tenant.id'],
Env      LowCardinality(String) MATERIALIZED ResourceAttributes['deployment.environment.name'],
...
ORDER BY (TenantId, Env, ServiceName, toDateTime(Timestamp))
```

Rules:

- The schema is in Git. The exporter does not create tables (`create_schema: false`).
- `TenantId` is first in the sorting key. Queries for one tenant read only the granules of that tenant.
- All tenants share one `otel_traces` table and one `otel_logs` table in each tier.
  ClickHouse recommends shared tables for tenants with the same schema and moderate volumes ([multi-tenancy guide](https://clickhouse.com/docs/products/cloud/guides/best-practices/multitenancy)).

### Retention

Retention uses TTL expressions with a condition for each retention class:

```sql
ALTER TABLE otel.otel_traces MODIFY TTL
  toDateTime(Timestamp) + INTERVAL 7  DAY DELETE WHERE Env IN ('dev', 'at22'),
  toDateTime(Timestamp) + INTERVAL 14 DAY DELETE WHERE Env = 'perf',
  toDateTime(Timestamp) + INTERVAL 30 DAY DELETE;
```

Retention for a customer comes from its contract. The values are in Git.

### Backup

We back up each cluster to Azure Blob Storage with `BACKUP ... TO AzureBlobStorage(...)`.
The pilot decides the schedule and the recovery point objective (RPO).

## Ingestion

### Edge collector

The edge collector uses the image `ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-contrib`.
It uses the Helm chart from `https://open-telemetry.github.io/opentelemetry-helm-charts`.
Changes to the current setup:

- The service account gets the annotation `azure.workload.identity/client-id`.
- The pod gets the label `azure.workload.identity/use: "true"`.
- The `azure_auth` extension gets a token for the audience of the tier.
- The `otlphttp` exporter uses `zstd` compression and a persistent sending queue.

```yaml
extensions:
  azure_auth:
    workload_identity:
      client_id: ${env:AZURE_CLIENT_ID}
      tenant_id: ${env:AZURE_TENANT_ID}
      federated_token_file: ${env:AZURE_FEDERATED_TOKEN_FILE}
    scopes: ["api://otel-ingest-prod/.default"]
  file_storage/queue:
    directory: /var/lib/otelcol/queue
```

We use OTLP over HTTP, not gRPC. OTLP over HTTP passes through all ingress controllers and web application firewalls.

### Ingest gateway

There is one ingest gateway for each tier. Each gateway has at least two replicas.
A tenant with a very large volume can get its own gateway. The Flux `ResourceSet` creates it from an input.

```yaml
extensions:
  oidc:
    providers:
      - issuer_url: https://login.microsoftonline.com/<entra-tenant-id>/v2.0
        audience: "<otel-ingest-prod-client-id>"

receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318
        auth:
          authenticator: oidc

processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80
    spike_limit_percentage: 20
  resource/identity:
    attributes:
      - { key: ingest.role, from_context: auth.claims.roles, action: upsert }
      - { key: telemetry.sender.oid, from_context: auth.claims.oid, action: upsert }
  transform/identity:
    trace_statements:
      - merge_maps(resource.attributes, ExtractPatterns(resource.attributes["ingest.role"][0], "^ingest\\.(?P<tenant>[a-z0-9-]+)\\.(?P<env>[a-z0-9-]+)$"), "upsert")
      - set(resource.attributes["tenant.id"], resource.attributes["tenant"])
      - set(resource.attributes["deployment.environment.name"], resource.attributes["env"])
      - delete_matching_keys(resource.attributes, "^(tenant|env|ingest\\.role)$")
    log_statements:
      # Same statements as trace_statements.
  filter/no-tenant:
    error_mode: ignore
    traces:
      span: ['resource.attributes["tenant.id"] == nil']
    logs:
      log_record: ['resource.attributes["tenant.id"] == nil']
  redaction/pii:
    # Remove tokens, e-mail addresses and national identity numbers.
  batch:
    send_batch_size: 10000
    timeout: 5s

exporters:
  clickhouse:
    endpoint: tcp://clickhouse-prod.clickhouse.svc:9000
    database: otel
    username: otel_writer_prod
    password: ${env:CLICKHOUSE_PASSWORD}
    create_schema: false

service:
  extensions: [health_check, oidc]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, resource/identity, transform/identity, filter/no-tenant, redaction/pii, batch]
      exporters: [clickhouse]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, resource/identity, transform/identity, filter/no-tenant, redaction/pii, batch]
      exporters: [clickhouse, otlphttp/sentinel-forwarder]
```

Rules:

- `resource/identity` must run before `batch`. The `batch` processor removes the request context.
- The gateway must overwrite `tenant.id` and `deployment.environment.name`. A sender cannot set them.
- The gateway drops data without a valid ingest role.
- The pilot must test the extraction of the first value of the `roles` claim.

### GitOps layout

A Flux Operator `ResourceSet` creates the gateways and the ClickHouse clusters from inputs:

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: otel-ingest-gateways
  namespace: otel
spec:
  inputs:
    - { tier: prod,    audience: "<prod-client-id>",    clickhouse: clickhouse-prod }
    - { tier: nonprod, audience: "<nonprod-client-id>", clickhouse: clickhouse-nonprod }
  resources:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      metadata:
        name: otel-ingest-<< inputs.tier >>
        namespace: otel
      spec:
        # Chart: open-telemetry/opentelemetry-collector. Values as in the gateway configuration above.
```

## Authentication

### Entra resources

Terraform creates these resources:

| Resource | Count | Purpose |
|---|---|---|
| `azuread_application` `otel-ingest-<tier>` | 1 for each tier | The token audience. It contains one ingest role for each tenant and environment. |
| `azuread_service_principal` | 1 for each tier | `app_role_assignment_required = true`. |
| `azurerm_user_assigned_identity` | 1 for each sending cluster | The identity of the edge collector. |
| `azurerm_federated_identity_credential` | 1 for each sending cluster | Trusts the service account token of the cluster. |
| `azuread_app_role_assignment` | 1 for each sending cluster | Gives the identity exactly one ingest role. |

Rules:

- `app_role_assignment_required` must be `true`. Without it, any identity in the Entra tenant can get a token.
- `requested_access_token_version` must be `2`. Version 1 tokens have a different issuer and the `oidc` extension rejects them.
- An identity must have exactly one ingest role.
- The identity of a `nonprod` cluster must not have a role on the `prod` application.
- The user-assigned identities are in our Entra tenant.
  A federated credential can trust a cluster in a customer subscription or in a customer Entra tenant.

### Gateway validation

The gateway validates tokens with the `oidc` extension.
The gateway must not use `azure_auth` in a receiver.
Versions v0.124.0 to v0.150.0 have a server-side authentication bypass in that mode ([GHSA-pjv4-3c63-699f](https://github.com/open-telemetry/opentelemetry-collector-contrib/security/advisories/GHSA-pjv4-3c63-699f)).

### Senders without workload identity

A sender without workload identity, for example a virtual machine, can still use Entra ID.
It uses `azure_auth` with a service principal and a client certificate.

### Authentication without Entra ID

Some senders cannot use Entra ID. Examples:

- A customer does not allow identities from our Entra tenant.
- A sender is outside Azure.
- We need a fallback during an Entra ID outage.

The gateway supports these methods for such senders. Each method uses a separate receiver on a separate ingress host.
TLS is mandatory for all methods.

| Method | How the gateway finds the tenant | Secret | Use |
|---|---|---|---|
| Basic authentication (`basicauth` extension, htpasswd) | The user name is the ingest role, for example `ingest.acme.prod`. The extension exposes the user name in the request context. | One password for each sender. bcrypt hash in the htpasswd file. | Default method without Entra ID. |
| Mutual TLS (mTLS) | The collector does not expose the certificate subject to processors. The receiver has a fixed tenant and environment. | Client certificate from cert-manager. | A sender that must not use passwords. |
| Static bearer token (`bearertokenauth` extension) | The receiver has a fixed tenant and environment. | One token for each receiver. | Pilot and tests only. |

Basic authentication on the gateway:

```yaml
extensions:
  basicauth/ingest:
    htpasswd:
      file: /etc/otel/htpasswd/ingest   # From Azure Key Vault through the External Secrets Operator.

receivers:
  otlp/basicauth:
    protocols:
      http:
        endpoint: 0.0.0.0:4319
        auth:
          authenticator: basicauth/ingest

processors:
  resource/identity-basicauth:
    attributes:
      - { key: ingest.role, from_context: auth.username, action: upsert }
  transform/identity-basicauth:
    trace_statements:
      - merge_maps(resource.attributes, ExtractPatterns(resource.attributes["ingest.role"], "^ingest\\.(?P<tenant>[a-z0-9-]+)\\.(?P<env>[a-z0-9-]+)$"), "upsert")
      # The other statements are the same as in transform/identity.
```

The `otlp/basicauth` receiver has its own pipelines.
These pipelines use `resource/identity-basicauth` and `transform/identity-basicauth`.
The other processors and the exporters are the same as in the Entra ID pipelines.

Basic authentication on the edge collector:

```yaml
extensions:
  basicauth/central:
    client_auth:
      username: ingest.acme.prod
      password_file: /etc/otel/secrets/ingest-password   # The collector reads the file again when it changes.
```

Rules:

- A password must have a minimum of 32 random characters.
- Terraform creates the passwords and writes them to Azure Key Vault.
- We change a password every 90 days. The edge collector reads the new password from the file without a restart.
- One sender has one user name. Two senders do not share a user name.
- Entra ID is the default. A sender uses one of these methods only when it cannot use Entra ID.

## Network

- The ingress of the gateways has a private IP address.
- Our own clusters connect through virtual network (VNet) peering.
- Customer clusters connect through Azure Private Link Service.
- A Kubernetes NetworkPolicy allows traffic to ClickHouse only from the gateways, Grafana and the agent proxy.

## Access control

### ClickHouse users and roles

```sql
-- One writer for each tier. The writer can only insert.
CREATE USER otel_writer_prod IDENTIFIED WITH sha256_password BY '...';
GRANT INSERT ON otel.* TO otel_writer_prod;

-- One reader role for each tenant.
CREATE ROLE tenant_acme;
GRANT SELECT ON otel.otel_traces, otel.otel_logs TO tenant_acme;
CREATE ROW POLICY acme ON otel.otel_traces USING TenantId = 'acme' TO tenant_acme;
CREATE ROW POLICY acme ON otel.otel_logs   USING TenantId = 'acme' TO tenant_acme;

-- The Grafana user of the tenant Grafana instance.
CREATE USER grafana_acme IDENTIFIED WITH sha256_password BY '...'
  SETTINGS PROFILE grafana_profile DEFAULT ROLE tenant_acme;
```

- The access configuration is in Git. Flux applies it as configuration files of the operator.
- Passwords come from Azure Key Vault through the External Secrets Operator.
- `query_log` records all queries with the user name. We keep `query_log` for 90 days.

### Grafana

We run Grafana OSS in the monitoring cluster. The [Grafana Operator](https://grafana.github.io/grafana-operator/) manages all Grafana instances and all Grafana resources.
We do not make manual changes in the Grafana UI.

#### One Grafana instance for each tenant

Each tenant gets its own Grafana instance in its own namespace (`grafana-<tenant>`). We do not use Grafana organizations.

- Grafana OSS does not have data source permissions. All users in an organization can query all data sources of that organization ([Grafana docs](https://grafana.com/docs/grafana/latest/administration/data-source-management/)).
  For this reason, a tenant must not share an organization with another tenant.
- The Grafana Operator does not manage Grafana organizations.
  The operator documentation recommends separate Grafana instances instead of organizations.
- A separate instance gives a separate URL, a separate sign-in configuration and a separate failure domain for each tenant.

Our own platform gets the instances `platform-prod` and `platform-nonprod`. The SOC gets the instance `soc`.

#### Resources for each instance

A Flux Operator `ResourceSet` creates these resources for each tenant from one input:

| Resource | Content |
|---|---|
| `Namespace` | `grafana-<tenant>`. |
| `Grafana` | The instance. Ingress host `<tenant>.grafana.example.com`. Entra ID sign-in. Database on the shared PostgreSQL server. |
| `GrafanaDatasource` | `clickhouse` (user `grafana_<tenant>`), `clickhouse-alerting` (user `alerting_<tenant>`) and `prometheus` (the Azure Monitor workspace of the tenant). The ClickHouse data sources install the plugin `grafana-clickhouse-datasource`. |
| `GrafanaFolder` | `otel` for dashboards. `alerts` for alert rules. |
| `GrafanaDashboard` | The OTel Traces Explorer, the OTel Service Dashboard and the OTel Logs Explorer. |
| `GrafanaAlertRuleGroup` | The standard alert rules. Refer to [Alerting on logs and traces](#alerting-on-logs-and-traces). |
| `GrafanaContactPoint` | The notification targets of the tenant. |
| `GrafanaNotificationPolicy` | One for each instance. A Grafana instance has only one notification policy. |
| `GrafanaServiceAccount` | `mcp-grafana` with the role `Viewer`. The operator writes the token to a Kubernetes secret in the same namespace. |

```yaml
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: grafana-tenants
  namespace: grafana-system
spec:
  inputs:
    - { tenant: platform-prod,    tier: prod,    entraGroup: "<group-object-id>", replicas: 2 }
    - { tenant: platform-nonprod, tier: nonprod, entraGroup: "<group-object-id>", replicas: 2 }
    - { tenant: acme,             tier: prod,    entraGroup: "<group-object-id>", replicas: 2 }
  resources:
    - apiVersion: grafana.integreatly.org/v1beta1
      kind: Grafana
      metadata:
        name: grafana
        namespace: grafana-<< inputs.tenant >>
        labels:
          tenant: << inputs.tenant >>
      spec:
        deployment:
          spec:
            replicas: << inputs.replicas >>
        config:
          server:
            root_url: https://<< inputs.tenant >>.grafana.example.com
          auth.azuread:
            enabled: "true"
            allowed_groups: << inputs.entraGroup >>
          database:
            type: postgres
            name: grafana_<< inputs.tenant >>
          unified_alerting:
            ha_peers: grafana-alerting.grafana-<< inputs.tenant >>.svc:9094
    - apiVersion: grafana.integreatly.org/v1beta1
      kind: GrafanaDatasource
      metadata:
        name: clickhouse
        namespace: grafana-<< inputs.tenant >>
      spec:
        instanceSelector:
          matchLabels:
            tenant: << inputs.tenant >>
        plugins:
          - name: grafana-clickhouse-datasource
            version: "4.23.0"
        valuesFrom:
          - targetPath: secureJsonData.password
            valueFrom:
              secretKeyRef: { name: clickhouse-grafana, key: password }
        datasource:
          name: ClickHouse
          type: grafana-clickhouse-datasource
          jsonData:
            host: clickhouse-<< inputs.tier >>.clickhouse.svc
            port: 9000
            protocol: native
            username: grafana_<< inputs.tenant >>
            defaultDatabase: otel
    # GrafanaFolder, GrafanaDashboard, GrafanaAlertRuleGroup, GrafanaContactPoint,
    # GrafanaNotificationPolicy and GrafanaServiceAccount follow the same pattern.
```

#### Rules

- The Grafana Operator version must be 5.24.0 or later. Earlier versions have a privilege escalation through Jsonnet dashboards.
- Each instance uses its own database on a shared Azure Database for PostgreSQL server. The `Database` resource from RFC 0013 can create these databases.
- An instance that sends alerts runs with at least two replicas. Grafana alerting runs in high availability mode, so that the replicas do not send the same notification two times.
- Users sign in with Entra ID. Customer users are Entra B2B guests in our Entra tenant. An Entra group gives access to the instance of one tenant.
- Terraform creates the Entra application for Grafana sign-in, the Entra groups and the workload identity of each instance. The Grafana Operator cannot create Entra resources.

#### Ownership: Terraform and the Grafana Operator

Each object has exactly one owner. Terraform does not call the Grafana API.

| Owner | Objects |
|---|---|
| Terraform | The Entra application for Grafana sign-in, Entra groups, B2B guest invitations, the workload identity of each instance, the role `Monitoring Data Reader` on Azure Monitor workspaces, Azure Key Vault and passwords, the PostgreSQL server for Grafana. |
| Grafana Operator | Grafana instances, data sources, folders, dashboards, alert rules, contact points, notification policies and service accounts. |
| Grafana configuration in the `Grafana` resource | Entra ID sign-in and the mapping of Entra groups to Grafana roles. We do not use Grafana teams or Grafana users. |

The handoff from Terraform to Flux:

1. Terraform writes the identifiers (client IDs, group IDs) to Azure Key Vault or to a ConfigMap.
2. The External Secrets Operator or Flux `postBuild.substituteFrom` gives the values to the `ResourceSet`.
3. The Grafana Operator applies the values to the Grafana instances.

If the pilot finds a gap in the Grafana Operator, for example teams or folder permissions, Terraform can own that object type.
This RFC must then name the object type and its owner.
- The Prometheus data source uses the workload identity of the instance with the role `Monitoring Data Reader` on the Azure Monitor workspace of the tenant.
- Secrets come from Azure Key Vault through the External Secrets Operator. The `GrafanaDatasource` reads them with `valuesFrom`.

### Alerting on logs and traces

Grafana alert rules query ClickHouse. The ClickHouse plugin supports alert rules.
Alerting is part of the pilot. Refer to [Pilot](#pilot).

Example: the ratio of failed server spans for each service in the last 5 minutes.

```sql
SELECT
  ServiceName,
  countIf(StatusCode = 'Error') / count() AS error_ratio
FROM otel.otel_traces
WHERE Timestamp >= now() - INTERVAL 5 MINUTE
  AND SpanKind = 'Server'
GROUP BY ServiceName
```

Each row becomes one alert instance with the label `ServiceName`.
Other alert types for the pilot:

| Alert | Source | Condition |
|---|---|---|
| Error ratio | `otel_traces` | `error_ratio > 0.05` for 5 minutes. |
| Latency | `otel_traces` | p95 of `Duration` for a route is higher than a limit for 10 minutes. |
| Error logs | `otel_logs` | The count of `SeverityText = 'ERROR'` for a service is higher than a limit. |
| Log pattern | `otel_logs` | `Body` matches a pattern, for example `OutOfMemory`. |
| No data | `otel_traces` | A service sends no spans for 15 minutes. |

Rules:

- The Grafana Operator manages the alert rules (`GrafanaAlertRuleGroup`), the contact points (`GrafanaContactPoint`) and the notification policy (`GrafanaNotificationPolicy`).
- Each tenant Grafana instance has its own rules and contact points.
- Each instance has a separate ClickHouse data source for alerting.
  This data source uses the user `alerting_<tenant>`, with the profile `alerting_profile`.
  The profile limits the run time of a query to 10 seconds.
- An alert query must filter on `Timestamp`. A query without a time filter reads the full table.
- The evaluation interval of an alert rule is 1 minute or longer.
- If an alert query is too slow, we add a materialized view that keeps counts for each minute.

### AI agents

- `mcp-grafana` uses the `GrafanaServiceAccount` token of one tenant Grafana instance. The ClickHouse tools are off by default. We enable them with `--enabled-tools`.
- `mcp-clickhouse` runs queries with `readonly = 1`. The server enforces more limits:

```sql
CREATE SETTINGS PROFILE agent_profile SETTINGS
  readonly = 1,
  max_execution_time = 30        READONLY,
  max_memory_usage   = 4000000000 READONLY,
  max_rows_to_read   = 2000000000 READONLY,
  max_result_rows    = 10000     READONLY,
  max_threads        = 4         READONLY;

CREATE QUOTA agent_quota FOR INTERVAL 1 hour
  MAX queries = 500, read_rows = 50000000000 TO ai_agent_acme;
```

- An agent user only gets `SELECT` on masked views. A masked view removes or hashes sensitive attributes.
- An agent user is limited to one tenant. An internal operations agent can read more tenants, but only through masked views.
- The `redaction` processor in the gateway is the first protection. The masked views are the second protection.

## Metrics

Metrics stay in Azure Monitor managed service for Prometheus.

- Ingestion costs 0.16 USD for each 10 million samples. Retention of 18 months is included.
- 100,000 active series with a 30-second scrape interval cost approximately 140 USD/month.
- Grafana gets a Prometheus data source for each Azure Monitor workspace.
  Grafana uses workload identity with the role `Monitoring Data Reader` on that workspace.
- The `spanmetrics` connector can create many series. Do not use high-cardinality dimensions.
  Prefer RED queries on ClickHouse.

## PostgreSQL

| Source | Path |
|---|---|
| Azure Database for PostgreSQL audit and connection logs | Diagnostic settings to Sentinel. No change. |
| Azure Database for PostgreSQL server logs for troubleshooting | Diagnostic settings to Azure Event Hubs, then the `azureeventhub` receiver in the gateway. |
| PostgreSQL in AKS | The `filelog` receiver in the edge collector and the `postgresql` receiver for metrics. |

## Security and the SOC

| Data | Destination |
|---|---|
| Entra, Azure Activity, Defender, AKS audit, PostgreSQL audit, web application firewall | Sentinel, through native connectors and diagnostic settings. No change. |
| Application security events (`event.domain = security`, failed authentication) | Sentinel custom table and ClickHouse. |
| All other application logs and traces | ClickHouse. Optional copy to the Sentinel data lake tier. |

- The gateway sends security events to a Sentinel custom table through the Logs Ingestion API.
  The contrib collector does not have an exporter for this API.
  A small forwarder (`otlphttp/sentinel-forwarder`) receives OTLP and calls the API.
- The SOC must give a list of analytics rules that read `AppTraces`, `AppRequests`, `AppExceptions` or `ContainerLogV2`.
  We do not move a table before the SOC accepts the change.
- The SOC gets a Grafana instance with read access to all tenants through masked views.

## Pilot

The pilot runs for 4–6 weeks on our own platform. Customers are not part of the pilot.
The edge collectors send to the ingest gateways and to Application Insights at the same time.

Scope:

| Area | The pilot must show |
|---|---|
| Ingestion | All six environments send traces and logs. No data is lost during a restart of the monitoring cluster. |
| Authentication | Entra ID workload identity works for all clusters. One sender uses basic authentication. |
| Isolation | A tenant Grafana instance and an agent user cannot read rows of another tenant or environment. |
| Grafana Operator | The Grafana Operator creates all Grafana instances and resources from Git. No manual changes are necessary. |
| UI | Developers can find traces, follow links between traces and logs, and use the OTel dashboards. |
| Alerting | Grafana alert rules on ClickHouse queries fire for all alert types in [Alerting on logs and traces](#alerting-on-logs-and-traces). Notifications go to the team channels. |
| AI agents (high priority) | `mcp-grafana` and `mcp-clickhouse` work within the limits of the settings profile and the quota. An agent cannot write data, cannot read another tenant and cannot read unmasked sensitive data. `query_log` shows each agent query. |
| SOC | Application security events arrive in the Sentinel custom table. |
| Performance | A load test in the large test environment does not slow down queries on `prod` data. |
| Operations | Backup and restore work. We know the real compression ratio and the disk use for each day. |
| Operator | We select the ClickHouse operator. |

Exit criteria:

- All rows in the scope table pass.
- An alert on a ClickHouse query fires within 2 minutes of the condition.
- A test with a malicious agent prompt does not read data of another tenant, does not change data and does not exceed the quota.
- The teams accept the Grafana views as a replacement for Application Insights.

## Cost estimate

All prices are list prices without the Enterprise Agreement discount. Prices in Norway East can be higher.

Today, for our own platform only:

| Item | GB/day | USD/month |
|---|---|---|
| `prod` (100 GB/day commitment tier) | ~107 | ~6,300 |
| Large test environment (commitment tier) | ~107 | ~6,300 |
| Four smaller environments (pay-as-you-go) | ~60 | ~4,200–5,000 |
| **Total** | ~275 | **~15,000–17,500** |

Proposal, for our own platform only:

| Item | USD/month |
|---|---|
| ClickHouse `prod` (nodes and disks) | ~1,150 |
| ClickHouse `nonprod` | ~600–1,100 |
| ClickHouse Keeper | ~150–250 |
| Ingest gateways | ~300–450 |
| Grafana, backups, network | ~150–300 |
| Operations (0.25–0.5 full-time equivalent) | ~2,500–5,000 |
| **Total** | **~5,000–8,000** |

- The savings increase with retention longer than 90 days and with customer volumes.
- Reserved instances for the ClickHouse nodes decrease the compute cost by 30–40 %.
- Sentinel-enabled workspaces pay more for each GB. Moving traces out of these workspaces saves more than this table shows.

## Corner cases

| Case | Behaviour |
|---|---|
| The monitoring cluster is not available. | The edge collector keeps the data in its persistent queue and sends it later. Data is lost when the queue is full. |
| A token expires during a request. | The gateway rejects the request with HTTP 401. The exporter gets a new token and sends again. |
| An identity has no ingest role or two ingest roles. | The gateway drops the data. An alert fires on `otelcol_processor_filter_*` metrics. |
| A sender sets `tenant.id` to another tenant. | The gateway overwrites the value with the tenant from the token. |
| One tenant sends a very large volume. | The tenant gets its own gateway from a `ResourceSet` input. Sampling at the edge is a second option. |
| A schema change is necessary. | We apply the DDL from Git before we change the exporter. |
| A customer leaves. | We remove the role assignments, delete the rows and remove the Grafana instance. |

# Drawbacks
[drawbacks]: #drawbacks

- We operate a stateful database. This needs ClickHouse knowledge, on-call and upgrades.
- We lose features of Application Insights: the application map, smart detection and availability tests.
  We must build replacements in Grafana.
- We become the data processor for customer telemetry. This needs data processing agreements and deletion procedures.
- The monitoring cluster is a shared dependency for all tenants.
- We operate approximately 78 Grafana instances. Upgrades go through one `ResourceSet`, but each instance uses memory and a database.
- Users who work across tenants must use more than one Grafana URL.
- The official ClickHouse operator has a `v1alpha1` API. The API can change.
- The Logs Ingestion API forwarder for Sentinel is new code that we must maintain.
- Basic authentication adds passwords that we must create, store and change.
- Alert queries add load on ClickHouse. The load grows with the number of tenants and rules.

# Rationale and alternatives
[rationale-and-alternatives]: #rationale-and-alternatives

## Why ClickHouse

- Grafana has an official ClickHouse plugin with OTel trace views and OTel dashboards.
- SQL gives a strong fallback for heavy analytical queries.
- ClickHouse enforces settings profiles, quotas, row policies and column grants for each user.
  This gives the strongest control for AI agents and for tenants.
- ClickHouse can move old data to Azure Blob Storage for long retention.

## Alternatives

| Alternative | Why we did not choose it |
|---|---|
| Stay on Application Insights. Use Basic logs or a dedicated cluster. | The cost decreases, but the query model and the UI stay the same. Agent access stays limited. |
| VictoriaTraces and VictoriaLogs | Low operations effort. The self-hosted documentation for VictoriaTraces says that it is not ready for production. There is no SQL, and limits apply to the whole server, not to each user. |
| Grafana Tempo and Loki | Good trace UI. Grafana Labs moved the OSS Helm charts to a community repository. Tempo 3.0 needs Kafka for high availability. Isolation is for each tenant only. |
| SigNoz | A full APM UI on ClickHouse. The enterprise directories have a separate licence. We get less control over the schema. |
| Jaeger v2 with ClickHouse | The ClickHouse backend is alpha. We can add the Jaeger UI later on the same ClickHouse. |
| OpenObserve | A single binary with object storage. We found little independent production experience. |
| One database for each tenant | Possible for 75 tenants. Gives smaller inserts, more parts and more schema migrations. |
| One gateway for each tenant and environment | More than 400 deployments. Shared gateways with ingest roles give the same isolation. |
| One Grafana instance with one organization for each tenant | Fewer pods. The Grafana Operator does not manage organizations, so we must manage them with other tools. A fault in the instance stops all tenants. |
| Grafana Enterprise for data source permissions | Adds a licence cost. Separate instances give enough isolation. |
| Terraform Grafana provider for all Grafana objects | Refer to [Grafana Operator and Terraform Grafana provider](#grafana-operator-and-terraform-grafana-provider). |
| Managed ClickStack (ClickHouse Cloud) | Removes operations effort. Data leaves our Azure tenant, and the cost model is different. We can test it later. |

## Grafana Operator and Terraform Grafana provider

| Property | Grafana Operator | Terraform Grafana provider |
|---|---|---|
| Reconciliation | Continuous. The operator removes manual changes. | Only at `apply`. Manual changes stay until the next `plan`. |
| Fit with the platform | Kubernetes resources in Git. Flux reconciles them. | A separate pipeline and a separate state. |
| A template for each tenant | One `ResourceSet` input for each tenant. | A provider block cannot use `for_each` in Terraform. 78 instances need 78 provider aliases. OpenTofu 1.9 and later can use `for_each` on providers. |
| Bootstrap | One step. The operator creates the instance and its resources. | Two steps. The instance must exist and be reachable before the provider can configure it. |
| Network access | The operator is in the cluster. | The CI runner needs network access and admin credentials for each instance. |
| Secrets | `valuesFrom` reads Kubernetes secrets. | Secrets are in the Terraform state. |
| Service account tokens | `GrafanaServiceAccount` writes the token to a Kubernetes secret. | The token is in the Terraform state. We must distribute it. |
| Ownership by teams | A team can own dashboards and alert rules in its own namespace. | All changes go through the central pipeline. |
| API coverage | No organizations, teams or users. Coverage is behind the Grafana API. | Almost the full API: organizations, teams, folder permissions, SSO settings and alerting. |
| Preview of changes | No `plan`. Errors show in the status of the resource after the change. | `plan` shows the difference before the change. |
| External Grafana, for example Azure Managed Grafana | Possible as an external instance, with limits. | Full support. |
| Azure and Entra resources | Not possible. | The same plan can refer to Entra groups, identities and role assignments. |
| Security | A controller with cluster permissions. Versions 5.23 and earlier have a privilege escalation. | Admin credentials in CI. |

Decision: Terraform owns all objects outside Grafana. The Grafana Operator owns all objects inside Grafana.
Refer to [Ownership: Terraform and the Grafana Operator](#ownership-terraform-and-the-grafana-operator).

## Impact of not doing this

- The cost of Application Insights grows with the volume.
- Each customer continues to have its own fragmented set of tools.
- AI agents cannot use logs and traces in a safe way.

# Prior art
[prior-art]: #prior-art

- [ClickStack](https://clickhouse.com/docs/use-cases/observability/clickstack/architecture) stores logs, metrics and traces in ClickHouse with the OTel exporter schema.
- [SigNoz](https://github.com/SigNoz/signoz) and Coroot use ClickHouse as the store for OTel data.
- [Jaeger v2.18](https://www.cncf.io/blog/2026/06/23/building-jaegers-clickhouse-backend-8-6x-compression-on-10-million-spans/) added ClickHouse as a storage backend. It got 8.6 times compression on 10 million spans.
- [LaunchDarkly](https://launchdarkly.com/docs/tutorials/row-level-security.md) uses ClickHouse row policies for tenant isolation of logs and traces.
- RFC 0004 (container runtime workload identity) is the base for workload identity in our clusters.
- RFC 0013 (multitenant DIS databases) uses a shared server for many tenants. This RFC uses the same pattern for telemetry.

# Unresolved questions
[unresolved-questions]: #unresolved-questions

To resolve in the RFC process:

- Do we accept the ownership of a stateful database, including on-call?
- Do we use the official ClickHouse operator or the Altinity operator?
- What is the daily volume for each customer, and the total?
- Are the customer subscriptions in our Entra tenant or in customer Entra tenants?
- Which Sentinel analytics rules read Application Insights or container log tables?
- What do the customer contracts say about retention, region and deletion?
- Where do the Grafana instances of customers run? Refer to [Placement of Grafana](#placement-of-grafana).

To resolve in the implementation:

- Does the extraction of the first `roles` value work in the `transform` processor?
- Which forwarder do we use for the Sentinel Logs Ingestion API?
- Which senders cannot use Entra ID and must use basic authentication?
- What is the ClickHouse load from alert queries for 75 tenants?
- What memory and database capacity do approximately 78 Grafana instances need?
- What are the backup schedule and the RPO?
- What is the real compression ratio on our data?

Out of scope:

- Moving metrics from managed Prometheus.
- Real user monitoring for browsers.

## Placement of Grafana

Grafana does not have to run in the same cluster as ClickHouse.
Grafana needs only a network path to ClickHouse.
ClickHouse enforces the isolation with row policies, settings profiles and quotas, so a remote Grafana cannot read more data than a local Grafana.

| Option | Advantages | Disadvantages |
|---|---|---|
| A. All Grafana instances run in the monitoring cluster. This is the design in this RFC. | ClickHouse is not exposed outside the cluster. One operator and one place for upgrades. Low latency. | Approximately 78 instances in one cluster. Customers reach Grafana through our ingress. |
| B. The Grafana instance of a customer runs in the AKS cluster of that customer. Flux and the Grafana Operator in that cluster manage it. | Grafana is in the network of the customer. Load and failures are spread over many clusters. The monitoring cluster runs only ClickHouse and the gateways. | ClickHouse is reachable from approximately 75 networks. Each cluster needs a certificate or a password. Grafana versions can drift. Alerting depends on the network path to ClickHouse. |
| C. The customer keeps Azure Managed Grafana. It connects to ClickHouse through a managed private endpoint to our Private Link Service. | We do not operate Grafana for the customer. | The same exposure as option B. We do not know if Azure Managed Grafana can use the ClickHouse plugin. The Grafana Operator can manage it only as an external instance. |

Remote access to ClickHouse (options B and C) needs these controls:

1. ClickHouse accepts remote connections only on a private load balancer, through Private Link Service or VNet peering. ClickHouse does not have a public IP address.
2. Remote connections use TLS on the native port (9440) or the HTTPS port (8443). The plain ports (9000 and 8123) are not reachable from outside the cluster.
3. A remote Grafana authenticates with a client certificate, for example `CREATE USER grafana_acme IDENTIFIED WITH ssl_certificate CN 'grafana-acme'`. cert-manager in the cluster of the Grafana instance issues the certificate.
4. Each remote user has a source address limit, for example `CREATE USER ... HOST IP '10.42.0.0/16'`.
5. Each alert rule sets the "No data" and "Error" states, so that a network failure does not cause an alert storm or a silent failure.
6. Grafana runs in the same Azure region as ClickHouse.

Proposal: we use option A for the pilot and for our own platform.
We configure TLS on the native port, certificate authentication and source address limits from the start.
Then a later move of a customer to option B changes only the data source of that customer.

Questions to resolve:

- Do customers want Grafana in their own cluster (option B), or keep Azure Managed Grafana (option C)?
- Can Azure Managed Grafana use the ClickHouse plugin?
- Does the Grafana ClickHouse plugin support client certificate authentication on the native port and on the HTTPS port?

# Future possibilities
[future-possibilities]: #future-possibilities

- Move data older than 30 days to Azure Blob Storage, for long retention at low cost.
- Add HyperDX or the Jaeger UI as a second UI on the same ClickHouse.
- Show the cost for each tenant from the row count and bytes in ClickHouse.
- Use tail sampling in the gateways to decrease the volume of successful, fast traces.
- Move metrics to a self-hosted VictoriaMetrics cluster with tenant support, if the managed Prometheus cost becomes high.
- Copy all application logs to the Sentinel data lake tier for SOC forensics.
