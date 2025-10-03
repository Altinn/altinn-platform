variable "psql_subnet_cidr" {
  type        = string
  description = "The CIDR range to use for psql subnet"
}

variable "organization" {
  type        = string
  description = "The organization/service owner name"
}

variable "environment" {
  type        = string
  description = "The environment (ATxx/TTx/YTx)"
}

variable "location" {
  type        = string
  default     = "norwayeast"
  description = "The location where the resources will be placed."
}

variable "psql_network_name" {
  type        = string
  description = "psql network name"
}

variable "psql_network_resource_group" {
  type        = string
  description = "psql network name"
}

variable "psql_resource_group" {
  type        = string
  description = "psql resource group"
}

variable "product" {
  type        = string
  description = "Product the resource belongs to"
}

variable "psql_compute_size" {
  type        = string
  description = "psql compute size"
}

variable "psql_high_availability_enabled" {
  type        = bool
  default     = true
  description = "Enable psql high availability"
}

variable "locks_off" {
  type        = bool
  default     = false
  description = "If locks should be on or off"
}

variable "psql_admin_group_ids" {
  description = "A list of EntraID group ids to be used as administrators"
  type        = list(string)
}

variable "psql_storage_size" {
  description = "PostgreSQL storage size in MB"
  type        = number
}

variable "psql_database_name" {
  description = "The name of the database"
  type        = string
}

variable "log_analytics_workspace_id" {
  description = "Log analytics workspace id"
  type        = string
}

variable "psql_storage_auto_grow" {
  type        = bool
  default     = true
  description = "Enable psql storage auto grow"
}

variable "psql_geo_redundant_backup_enabled" {
  type        = bool
  default     = true
  description = "Enable psql georedundant backup"
}

variable "psql_database_collation" {
  description = "The collation to use for the database"
  type        = string
}

variable "psql_server_name" {
  description = "The name of the psql server"
  type        = string
}
 
variable "psql_pgbouncer_enabled" {
  type        = bool
  default     = false
  description = "Enable pgbouncer for postgresql" 
}

variable "psql_pgbouncer_pool_mode" {
  type        = string
  default     = "transaction"
  description = "pgbouncer pool mode (SESSION, TRANSACTION, STATEMENT)"
}

variable "psql_enable_private_access" {
  description = "Set to true to deploy PostgreSQL with VNet integration (private access). False for public access."
  type        = bool
  default     = true
}

variable "psql_version" {
  description = "PostgreSQL version to use"
  type        = string
}

variable "psql_enable_virtual_endpoint" {
  description = "Set to true to deploy PostgreSQL with virtual endpoint"
  type        = bool
  default     = false
}

variable "psql_virtual_endpoint_name" {
  description = "The name of the PostgreSQL virtual endpoint"
  type        = string
}

variable "psql_maintenance_day_of_week" {
  type        = number
  default     = 2
  description = "1=Monday ... 7=Sunday (Azure spec)."
}

variable "psql_maintenance_start_hour" {
  type        = number
  default     = 1
  description = "0–23."
}

variable "psql_maintenance_start_minute" {
  type        = number
  default     = 0
  description = "0, 5, 10 ... 55 (increments of 5)."
}

variable "psql_extensions" {
  description = "Comma-separated PostgreSQL extensions (e.g. \"pg_trgm,pg_stat_statements\"). Empty = none."
  type        = string
  default     = ""
  validation {
    condition = (
      var.psql_extensions == "" ||
      length([
        for e in split(",", replace(var.psql_extensions, " ", "")) :
        e if (length(e) > 0 && can(regex("^[A-Za-z0-9_]+$", e)))
      ]) == length(split(",", replace(var.psql_extensions, " ", "")))
    )
    error_message = "psql_extensions must be comma-separated alphanumeric/underscore names."
  }
}

variable "psql_shared_preload_libraries" {
  description = "Comma-separated shared_preload_libraries (e.g. \"pg_stat_statements,pg_cron\"). Empty = none."
  type        = string
  default     = ""
  validation {
    condition = (
      var.psql_shared_preload_libraries == "" ||
      length([
        for e in split(",", replace(var.psql_shared_preload_libraries, " ", "")) :
        e if (length(e) > 0 && can(regex("^[A-Za-z0-9_]+$", e)))
      ]) == length(split(",", replace(var.psql_shared_preload_libraries, " ", "")))
    )
    error_message = "psql_shared_preload_libraries must be comma-separated alphanumeric/underscore names."
  }
}

variable "psql_custom_configurations" {
  description = "Custom PostgreSQL server configurations (name => value)."
  type        = map(string)
  default     = {}

  validation {
    condition = (
      length(var.psql_custom_configurations) <= 25
      &&
      length([
        for k in keys(var.psql_custom_configurations) :
        k if (
          can(regex("^[a-zA-Z0-9_\\.]+$", k))
          && !contains([
            "azure.extensions",
            "shared_preload_libraries",
            "pgbouncer.enabled",
            "pgbouncer.pool_mode"
          ], k)
        )
      ]) == length(var.psql_custom_configurations)
      &&
      length([
        for v in values(var.psql_custom_configurations) :
        v if length(trimspace(v)) > 0
      ]) == length(var.psql_custom_configurations)
    )
    error_message = "Max 25 configs. Keys must be alphanumeric/underscore/dot, not reserved (azure.extensions, shared_preload_libraries, pgbouncer.*), and values cannot be empty."
  }
}

variable "psql_track_actual_storage" {
  description = "If true, expose actual grown storage size (read-only) via AzAPI. Does not change storage_mb."
  type        = bool
  default     = false
}

variable "psql_storage_tier" {
  description = "Optional storage tier for the flexible server (e.g. P4, P6, P10 ...). Null = Azure default."
  type        = string
  default     = null
  validation {
    condition = (
      var.psql_storage_tier == null ||
      contains([
        "P4","P6","P10","P15","P20","P30","P40","P50","P60","P70","P80"
      ], var.psql_storage_tier)
    )
    error_message = "psql_storage_tier must be one of: P4,P6,P10,P15,P20,P30,P40,P50,P60,P70,P80 or null."
  }
}

variable "psql_backup_retention_days" {
  description = "Backup retention days (7–35). Defaults to 35."
  type        = number
  default     = 35
  validation {
    condition     = var.psql_backup_retention_days >= 7 && var.psql_backup_retention_days <= 35
    error_message = "psql_backup_retention_days must be between 7 and 35."
  }
}

variable "psql_firewall_rules" {
  description = "Map of firewall rules (name => { start_ip, end_ip }). Ignored if VNet integration enabled."
  type = map(object({
    start_ip = string
    end_ip   = string
  }))
  default = {}
}

variable "psql_diagnostics_enabled" {
  description = "Enable diagnostic settings for the PostgreSQL flexible server."
  type        = bool
  default     = true
}


variable "psql_diagnostic_log_categories" {
  description = "Friendly diagnostic log categories to enable. Empty liste = ingen logger."
  type        = list(string)
  default = [
    "PostgreSQL Server Logs",
    "PostgreSQL Query Store Runtime",
    "PostgreSQL Query Store Wait Statistics",
    "PostgreSQL Sessions data",
    "PostgreSQL Autovacuum and schema statistics",
    "PostgreSQL remaining transactions"
  ]
  validation {
    condition = length([
      for c in var.psql_diagnostic_log_categories : c
      if contains([
        "PostgreSQL Server Logs",
        "PostgreSQL Query Store Wait Statistics",
        "PostgreSQL Sessions data",
        "PostgreSQL Query Store Runtime",
        "PostgreSQL Autovacuum and schema statistics",
        "PostgreSQL remaining transactions"
      ], c)
    ]) == length(var.psql_diagnostic_log_categories)
    error_message = "En eller flere oppgitte kategorier er ikke gyldige friendly navn."
  }
}

variable "psql_diagnostic_metrics" {
  description = "List of metric categories to enable (typically [\"AllMetrics\"]). Empty list = disable metrics."
  type        = list(string)
  default     = ["AllMetrics"]
  validation {
    condition     = length([for m in var.psql_diagnostic_metrics : m if m != "AllMetrics"]) == 0
    error_message = "Only 'AllMetrics' is currently supported."
  }
}

variable "psql_subnet_name" {
  description = "Optional explicit name for the PostgreSQL subnet. Defaults to \"<psql_ServerName>-subnet\" when null."
  type        = string
  default     = null
}

variable "existing_private_dns_zone_id" {
  type        = string
  default     = null
  description = "Set if zone already exists; module will not create zone or link."
}
