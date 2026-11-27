# Terraform Module for Creating a PostgreSQL Server

## Prerequisites

- VNET to create the subnet for PostgreSQL if using VNET integration (the module can create the subnet)
- Entra ID group(s) for PostgreSQL admin access
- A Log Analytics Workspace

---

## Module Configuration

The following is an example configuration for using the PostgreSQL Terraform module.

Minimum required parameters:

Private access (VNet integration):

```hcl
module "psql" {
  source                            = "../modules/psql"
  environment                       = "at24"
  organization                      = "platform"
  product                           = "studio"
  psql_server_name                  = "platform-olki-at24-psql"
  psql_version                      = "16"
  psql_database_name                = "olki"
  psql_database_collation           = "nb_NO.utf8"
  psql_resource_group               = "altinnplatform-at24-rg"
  existing_private_dns_zone_id      = null
  psql_network_resource_group       = "altinnplatform-rg"
  psql_Nntwork_name                 = "at-platform-vnet"
  psql_subnet_name                  = "platform-olki-at24-subnet"
  psql_subnet_cidr                  = "10.127.1.0/24"
  psql_compute_size                 = "GP_Standard_D2s_v3"
  psql_storage_size                 = 32768
  psql_admin_group_ids              = ["143ed28a-6e6d-4ca0-8273-eecb9c1665ba"] #Altinn-30-Test-Operations
  log_analytics_workspace_id         = "/subscriptions/de41df22-8dd0-435b-98dd-6152cd371e92/resourceGroups/Altinn-rg/providers/Microsoft.OperationalInsights/workspaces/altinn-at24-law"
}
```

Public access (allowed IP addresses)

```hcl
module "psql" {
  source                            = "../modules/psql"
  environment                       = "at24"
  organization                      = "platform"
  product                           = "studio"
  psql_server_name                  = "platform-olki-at24-psql"
  psql_version                      = "16"
  psql_database_name                = "olki"
  psql_database_collation           = "nb_NO.utf8"
  psql_resource_group               = "altinnplatform-at24-rg"
  psql_enable_vnet_integration      = false
  psql_compute_size                 = "GP_Standard_D2s_v3"
  psql_storage_size                 = 32768
  psql_admin_group_ids              = ["143ed28a-6e6d-4ca0-8273-eecb9c1665ba"] #Altinn-30-Test-Operations
  log_analytics_workspace_id        = "/subscriptions/de41df22-8dd0-435b-98dd-6152cd371e92/resourceGroups/Altinn-rg/providers/Microsoft.OperationalInsights/workspaces/altinn-at24-law"
}
```

```hcl
module "psql" {
  source                            = "../modules/psql"
  environment                       = "at24"
  organization                      = "platform"
  product                           = "studio"
  location                          = "norwayeast"
  psql_server_ame                   = "platform-olki-at24-psql"
  psql_enable_virtual_endpoint      = true
  psql_virtual_endpoint_name        = "altinn-endpoint-olki-at24"
  psql_version                      = "16"
  psql_database_name                = "olki"
  psql_database_collation           = "nb_NO.utf8"
  psql_backup_retention_days        = 7
  psql_resource_group               = "altinnplatform-at24-rg"
  psql_enable_vnet_integration      = false
  psql_firewall_rules               = {
        ai-dev-LabVM   = { start_ip = "51.120.0.114", end_ip = "251.120.0.114" }
        terraformVM    = { start_ip = "51.13.85.174", end_ip = "51.13.85.174" }
  }
  psql_network_resource_group       = "altinnplatform-rg"
  psql_network_name                 = "at-platform-vnet"
  psql_subnet_name                  = "platform-olki-at24-subnet"
  psql_subnet_cidr                  = "10.127.1.0/24"
  psql_compute_size                 = "GP_Standard_D2s_v3"
  psql_storage_size                 = 32768
  psql_storage_tier                 = "P10"
  psql_high_availability_enabled    = true
  psql_geo_redundant_backup_enabled = true
  psql_storage_auto_grow            = true
  psql_admin_group_ids              = ["143ed28a-6e6d-4ca0-8273-eecb9c1665ba"] #Altinn-30-Test-Operations
  psql_pgbouncer_enabled            = false
  psql_pgbouncer_pool_mode          = "transaction"
  psql_extensions                   = "PG_TRGM,PG_STAT_STATEMENTS,PG_BUFFERCACHE,PG_CRON,PGAUDIT"
  psql_shared_preload_libraries     = "auto_explain,pg_cron,pg_stat_statements,pgaudit"
  psql_custom_configurations        = {
    log_min_duration_statement      = "750"
    log_statement                   = "ddl"
    log_connections                 = "on"
    log_disconnections              = "on"
    log_lock_waits                  = "on"
    log_checkpoints                 = "on"
    log_temp_files                  = "0"
    log_autovacuum_min_duration     = "0"
    autovacuum_vacuum_scale_factor  = "0.05"
    autovacuum_analyze_scale_factor = "0.05"
    autovacuum_naptime              = "30"
    work_mem                        = "32768"
    maintenance_work_mem            = "262144"
    effective_io_concurrency        = "64"
    wal_compression                 = "on"
    "pgaudit.log"                   = "WRITE,DDL"
    "pgaudit.log_parameter"         = "on" 
  }
  psql_maintenance_day_of_week      = 2
  psql_maintenance_start_hour       = 1
  psql_maintenance_start_minute     = 0
  psql_track_actual_storage         = true
  log_analytics_workspace_id        = "/subscriptions/de41df22-8dd0-435b-98dd-6152cd371e92/resourceGroups/Altinn-rg/providers/Microsoft.OperationalInsights/workspaces/altinn-at24-law"
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
```

---

## Input Variables

(R = Required, O = Optional)

-**environment** (R): Environment (e.g. `at24`).
-**organization** (R): Organization (e.g. `platform`).
-**product** (R): Product/application identifier (e.g. `studio`).
-**location** (O, default: `norwayeast`): Azure region.

-**psql_server_name** (R): PostgreSQL Flexible Server name.
-**psql_version** (R): PostgreSQL major version (e.g. `16`).
-**psql_database_name** (R): Initial database name.
-**psql_database_collation** (O, default: `nb_NO.utf8`): Database collation.
-**psql_backup_retention_days** (O, default: `7` or module default): Backup retention (7–35).

-**psql_resource_group** (R): Resource group for the server.
-**existing_private_dns_zone_id** (O): Use an existing `*.postgres.database.azure.com` private DNS zone (skip creation/link). If not set a (servername).private.postgres.database.azure.com private dns zone will be created.
-**psql_enable_vnet_integration** (O, default: `true`): Deploy in delegated subnet (private access). If `false`, public network.
  
Networking (required only when `psql_enable_vnet_integration = true` and subnet created here):
-**psql_network_resource_group** (R*): RG containing the VNet.
-**psql_network_name** (R*): Virtual Network name.
-**psql_subnet_name** (R*): Subnet name (created if absent).
-**psql_subnet_cidr** (R*): Subnet CIDR (e.g. `10.127.1.0/24`).

Firewall (public access mode):
-**psql_firewall_rules** (O): Map of named rules `{ name = { start_ip = "...", end_ip = "..." } }`.

Compute & storage:
-**psql_compute_size** (R): SKU (e.g. `GP_Standard_D2s_v3`).
-**psql_storage_size** (R): Storage in MB (e.g. `32768`).
-**psql_storage_tier** (O, default: `null`): Premium tier (e.g. `P10`) or null for Azure default.
-**psql_storage_auto_grow** (O, default: `true`).
-**psql_track_actual_storage** (O, default: `false`).

Reliability:
-**psql_high_availability_enabled** (O, default: `false` or module default).
-**psql_geo_redundant_backup_enabled** (O, default: `false`/`true` per module default).

Identity & access:
-**psql_admin_group_ids** (R): List of Entra ID (Azure AD) group object IDs for admin access.
-**locks_off** (O, default: `false`): Disable creation of management locks.

Diagnostics:
-**log_analytics_workspace_id** (R): Target Log Analytics Workspace ID.
-**psql_diagnostics_enabled** (O, default: `true`).
-**psql_diagnostic_log_categories** (O): List of log categories (filtered to supported).
-**psql_diagnostic_metrics** (O): List of metric categories (e.g. `["AllMetrics"]`).

PgBouncer:
-**psql_pgbouncer_enabled** (O, default: `false`).
-**psql_pgbouncer_pool_mode** (O, default: `transaction`): `session|transaction|statement`.

Extensions & config:
-**psql_extensions** (O, default: `""`): Comma-separated list (e.g. `PG_TRGM,PG_CRON`).
-**psql_shared_preload_libraries** (O, default: `""`).
-**psql_custom_configurations** (O): Map of additional server parameters.

Maintenance window:
-**psql_maintenance_day_of_week** (O, default: `2` = Tuesday).
-**psql_maintenance_start_hour** (O, default: `1`).
-**psql_maintenance_start_minute** (O, default: `0`).

Endpoint (optional feature):
-**psql_enable_virtual_endpoint** (O, default: `false`): Enable outbound virtual endpoint (if module supports).
-**psql_virtual_endpoint_name** (O): Virtual endpoint name (required if enabled).

Notes:
-Fields marked R* are conditionally required (only when VNet integration is enabled and subnet created by module).
-Provide `existing_private_dns_zone_id` to reuse a central private DNS zone; otherwise module will create one.

---

## Outputs

The module provides the following outputs:

| Output | Description |
|-------|-------------|
| `psql_server_name` | The name of the PostgreSQL Flexible Server. |
| `psql_server_fqdn` | Fully qualified domain name of the server. |
| `psql_server_id` | The resource ID of the PostgreSQL Flexible Server. |
| `psql_identity_id` | The ID of the User Assigned Managed Identity. |
| `psql_admin_group_object_ids` | Object IDs of Entra ID groups granted admin access. |
| `psql_database_name` | Name of the created (initial) database. |
| `psql_private_dns_zone_name` | Name of private DNS zone if created by module; null if external. |
| `psql_effective_private_dns_zone_id` | ID of the private DNS zone actually in use (created or supplied); null if not using private access. |
| `psql_actual_storage_mb` | Observed storage size in MB (may exceed initial if AutoGrow enabled). |
