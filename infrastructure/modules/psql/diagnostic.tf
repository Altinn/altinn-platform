locals {
  # Friendly -> API mapping
  psql_diag_category_map_friendly = {
    "PostgreSQL Server Logs"                     = "PostgreSQLLogs"
    "PostgreSQL Query Store Wait Statistics"     = "PostgreSQLFlexQueryStoreWaitStats"
    "PostgreSQL Sessions data"                   = "PostgreSQLFlexSessions"
    "PostgreSQL Query Store Runtime"             = "PostgreSQLFlexQueryStoreRuntime"
    "PostgreSQL Autovacuum and schema statistics"= "PostgreSQLFlexTableStats"
    "PostgreSQL remaining transactions"          = "PostgreSQLFlexDatabaseXacts"
  }

  psql_effective_log_categories = [
    for friendly in var.psql_diagnostic_log_categories :
    local.psql_diag_category_map_friendly[friendly]
  ]

  psql_effective_metric_categories = distinct(var.psql_diagnostic_metrics)
}

resource "azurerm_monitor_diagnostic_setting" "postgresql" {
  count                      = var.psql_diagnostics_enabled ? 1 : 0
  name                       = "psql-diag"
  target_resource_id         = azurerm_postgresql_flexible_server.psql.id
  log_analytics_workspace_id = var.log_analytics_workspace_id

  dynamic "enabled_log" {
    for_each = toset(local.psql_effective_log_categories)
    content {
      category = enabled_log.value
    }
  }

  dynamic "enabled_metric" {
    for_each = toset(local.psql_effective_metric_categories)
    content {
      category = enabled_metric.value
    }
  }
}
