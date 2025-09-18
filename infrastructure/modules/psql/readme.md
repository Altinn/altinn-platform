# Terraform Module for Creating a PostgreSQL Server

## Prerequisites

- VNET to create the subnet for PostgreSQL if using VNET integration (the module can create the subnet)
- Entra ID group(s) for PostgreSQL admin access
- A Log Analytics Workspace

---

## Module Configuration

The following is an example configuration for using the PostgreSQL Terraform module.

```hcl
module "psql" {
  source                            = "../modules/psql"
  environment                       = "at24"
  organization                      = "platform"
  product                           = "studio"
  location                          = "norwayeast"
  psql_ServerName                   = "platform-olki-at24-psql"
  psql_enable_endpoint              = true
  psql_endpoint_name                = "altinn-endpoint-olki-at24"
  psql_Version                      = "16"
  psql_dbname                       = "olki"
  psql_DatabaseCollation            = "nb_NO.utf8"
  psql_backup_retention_days        = 7
  psql_ResourceGroup                = "altinnplatform-at24-rg"
  psql_enable_vnet_integration      = false
  psql_firewall_rules               = {
        ai-dev-LabVM   = { start_ip = "51.120.0.114", end_ip = "251.120.0.114" }
        terraformVM    = { start_ip = "51.13.85.174", end_ip = "51.13.85.174" }
  }
  psql_NetworkResourceGroup         = "altinnplatform-rg"
  psql_NetworkName                  = "at-platform-vnet"
  psql_subnet_name                  = "platform-olki-at24-subnet"
  psql_SubnetCidr                   = "10.127.1.0/24"
  psql_ComputeSize                  = "GP_Standard_D2s_v3"
  psql_StorageSize                  = 32768
  psql_StorageTier                  = "P10"
  psql_HighAvailability             = false
  psql_GeoRedundantBackup           = true
  psql_StorageAutoGrow              = true
  psql_AdminGroups                  = ["Altinn-30-Test-Operations"]
  psql_pgbouncer                    = false
  psql_pgbouncer_pool_mode          = "transaction"
  psql_extensions                   = "PG_TRGM,PG_STAT_STATEMENTS,PG_BUFFERCACHE,PG_CRON"
  psql_shared_preload_libraries     = "auto_explain,pg_cron,pg_stat_statements"
  psql_custom_configurations        = {
    log_min_duration_statement          = "750"
    log_lock_waits                      = "on"
    autovacuum_vacuum_scale_factor      = "0.05"
    autovacuum_analyze_scale_factor     = "0.05"
    autovacuum_naptime                  = "30"
    work_mem                            = "32768"
    maintenance_work_mem                = "262144"
    effective_io_concurrency            = "64"
    wal_compression                     = "on"
  }
  psql_maintenance_day_of_week      = 2
  psql_maintenance_start_hour       = 1
  psql_maintenance_start_minute     = 0
  psql_track_actual_storage         = true
  log_analytics_workspace_name      = "altinn-at24-law"
  log_analytics_workspace_rg        = "Altinn-rg"
  psql_diagnostics_enabled          = true
  psql_diagnostic_log_categories    = [
    "PostgreSQL Server Logs",
    "PostgreSQL Query Store Runtime",
    "PostgreSQL Query Store Wait Statistics",
    "PostgreSQL Sessions data",
    "PostgreSQL Autovacuum and schema statistics",
    "PostgreSQL remaining transactions"
  ]
  psql_diagnostic_metrics           = ["AllMetrics"]
  locks_off                         = true
}

output "psql_private_dns_zone_name" {
  value = module.psql.psql_private_dns_zone_name
}
output "psql_id" {
  value = module.psql.psql_server_id
}
output "psql_fqdn" {
  value = module.psql.psql_server_fqdn
}
output "psql_UserAssignedIdentity_id" {
  value = module.psql.psql_identity_id
}
```

---

## Input Variables

- **source**: The path to the Terraform module.
- **environment**: The environment in which the PostgreSQL server is deployed (e.g., "at24").
- **organization**: The organization name (e.g., "platform").
- **product**: The product name (e.g., "studio").
- **psqlServerName**: The name of the PostgreSQL server.
- **psql_enable_endpoint**:
- **psql_endpoint_name**:
- **psql_Version**: PostgreSQL version
- **psql_dbname**: The name of the database.
- **psqlDatabaseCollation**: The collation for the database (e.g., "nb_NO.utf8").
- **location**: The Azure region where the resources will be deployed (e.g., "norwayeast").
- **psql_backup_retention_days**: Days to keep backup (7 to 35 days).
- **psql_ResourceGroup**: The resource group for the PostgreSQL server.
- **psql_enable_vnet_integration**: Whether to create PostgreSQL in a VNET (`true`/`false`).
- **psql_firewall_rules**: A map of firewall ip ranges to allow traffic.
- **psql_NetworkResourceGroup**: The resource group for the network resources.
- **psql_NetworkName**: The name of the virtual network.
- **psql_subnet_name**: The name of the subnet (The module will create this subnet)
- **psqlSubnetCidr**: The CIDR block for the subnet (e.g., "10.127.1.0/24").
- **psqlComputeSize**: The compute size for the PostgreSQL server (e.g., "B_Standard_B1ms").
- **psqlStorageSize**: The storage size for the PostgreSQL server in MB (e.g., `32768`).
- **psqlStorageTier**: Optional storage tier (e.g. P4, P6, P10...). Omit / null to use Azure default.
- **psqlHighAvailability**: Whether to enable high availability (`true`/`false`).
- **psqlGeoRedundantBackup**: Whether to enable geo-redundant backup (`true`/`false`).
- **psqlStorageAutoGrow**: Whether to enable auto-grow for storage (`true`/`false`).
- **psqlAdminGroups**: A list of Azure AD groups to be used as administrators.
- **log_analytics_workspace_name**: The name of the Log Analytics workspace.
- **log_analytics_workspace_rg**: The resource group for the Log Analytics workspace.
- **psql_diagnostics_enabled**: Whether to enable diagnostic logging
- **psql_diagnostic_log_categories**: Map of log categories to log.
- **psql_diagnostic_metrics**: Map of log categories to log.
- **psql_pgbouncer**: Whether to enable PgBouncer (`true`/`false`). Using port 6432.
- **psql_pgbouncer_pool_mode**: The pooling mode to be used by PgBouncer. Common values include "session", "transaction", or "statement". Determines how client connections are managed.
- **psql_extensions**: Comma-separated list of PostgreSQL extensions to enable via `azure.extensions` (e.g. `pg_trgm,pg_stat_statements,pg_cron`). Empty string = none.
- **psql_shared_preload_libraries**: Comma-separated list of libraries for `shared_preload_libraries` (e.g. `pg_stat_statements,pg_cron`). Only supported libraries; changes can require server restart. Empty string = none.
- **psql_custom_configurations**: Map of additional PostgreSQL server configuration parameters (name => value). Reserved keys (azure.extensions, shared_preload_libraries, pgbouncer.enabled, pgbouncer.pool_mode) are ignored. Values must be non-empty strings. Some settings may trigger a server restart—use cautiously. Example: `{ log_min_duration_statement = "500", idle_in_transaction_session_timeout = "60000" }`.
- **psql_maintenance_day_of_week**: 1=Monday ... 7=Sunday (Azure spec).
- **psql_maintenance_start_hour**: 0–23
- **psql_maintenance_start_minute**: 0, 5, 10 ... 55 (increments of 5).
- **psql_track_actual_storage**: Whether to track storage (`true`/`false`).
- **locks_off**: Whether to disable management locks (`true`/`false`).

---

## Outputs

The module provides the following outputs:

- **psql_private_dns_zone_name**: The name of the private DNS zone.
- **psql_id**: The ID of the PostgreSQL Flexible Server.
- **psql_fqdn**: The fully qualified domain name (FQDN) of the PostgreSQL Flexible Server.
- **psql_UserAssignedIdentity_id**: The ID of the user assigned managed identity.
