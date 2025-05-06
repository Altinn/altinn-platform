resource "azurerm_log_analytics_workspace" "obs" {
  name                = var.log_analytics_workspace_name != "" ? var.log_analytics_workspace_name : "${var.prefix}-${var.environment}-obs-law"
  resource_group_name = var.azurerm_resource_group_obs_name
  location            = var.location
  retention_in_days   = var.log_analytics_retention_days

  tags = var.tags   
}

resource "azurerm_monitor_workspace" "obs" {
  name                = var.monitor_workspace_name != "" ? var.monitor_workspace_name : "${var.prefix}-${var.environment}-obs-amw"
  resource_group_name = var.azurerm_resource_group_obs_name
  location            = var.location

  tags = var.tags
}

resource "azurerm_application_insights" "obs" {

  name                 = var.app_insights_name != "" ? var.app_insights_name : "${var.prefix}-${var.environment}-obs-appinsights"
  resource_group_name = var.azurerm_resource_group_obs_name
  location            = var.location
  workspace_id         = azurerm_log_analytics_workspace.obs.id
  application_type     = "web"
  retention_in_days    = 30

  tags = var.tags
}
