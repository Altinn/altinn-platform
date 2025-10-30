# Resource reuse logic is now handled directly in resources using try() pattern

# Reuse existing resources if names are provided
data "azurerm_resource_group" "existing" {
  count = var.azurerm_resource_group_obs_name != null ? 1 : 0
  name  = var.azurerm_resource_group_obs_name
}

data "azurerm_log_analytics_workspace" "existing" {
  count               = var.log_analytics_workspace_name != null ? 1 : 0
  name                = var.log_analytics_workspace_name
  resource_group_name = var.azurerm_resource_group_obs_name
}

data "azurerm_application_insights" "existing" {
  count               = var.app_insights_name != null ? 1 : 0
  name                = var.app_insights_name
  resource_group_name = var.azurerm_resource_group_obs_name
}

data "azurerm_monitor_workspace" "existing" {
  count               = var.monitor_workspace_name != null ? 1 : 0
  name                = var.monitor_workspace_name
  resource_group_name = var.azurerm_resource_group_obs_name
}
