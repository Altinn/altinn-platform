resource "azurerm_log_analytics_workspace" "apimlogs" {
  name                = "${var.prefix}-${var.environment}-apim-logs"
  location            = azurerm_resource_group.apim_rg.location
  resource_group_name = azurerm_resource_group.apim_rg.name
  sku                 = "PerGB2018"
  retention_in_days   = 30
  tags                = var.tags
}

resource "azurerm_application_insights" "appinsights" {
  name                = "${var.prefix}-${var.environment}-apim-appinsights"
  resource_group_name = azurerm_resource_group.apim_rg.name
  location            = azurerm_resource_group.apim_rg.location
  application_type    = "web"
  workspace_id        = azurerm_log_analytics_workspace.apimlogs.id
  tags                = var.tags
}

resource "azurerm_api_management_logger" "apimlogger" {
  name                = "default-apim-ai"
  api_management_name = azurerm_api_management.apim.name
  resource_group_name = azurerm_resource_group.apim_rg.name

  application_insights {
    instrumentation_key = azurerm_application_insights.appinsights.instrumentation_key
  }
}

resource "azurerm_monitor_diagnostic_setting" "apimdiagnostics_settings" {
  name                           = "${var.prefix}-${var.environment}-apim-diagnostics-settings"
  target_resource_id             = azurerm_api_management.apim.id
  log_analytics_workspace_id     = azurerm_log_analytics_workspace.apimlogs.id
  log_analytics_destination_type = "Dedicated"
  enabled_log {
    category = "GatewayLogs"
  }
  enabled_metric {
    category = "AllMetrics"
  }
}

resource "azurerm_api_management_diagnostic" "apim_application_insights" {
  identifier               = "default-applicationinsights"
  resource_group_name      = azurerm_resource_group.apim_rg.name
  api_management_name      = azurerm_api_management.apim.name
  api_management_logger_id = azurerm_api_management_logger.apimlogger.id

  # A sampling percentage of 0.0 means no successful requests are sampled. Only errors will be logged due to always_log_errors = true.
  sampling_percentage       = 0.0
  always_log_errors         = true
  log_client_ip             = false
  verbosity                 = "error"
  http_correlation_protocol = "W3C"

  frontend_request {
    body_bytes     = var.body_bytes_to_log
    headers_to_log = var.headers_to_log
  }
}
