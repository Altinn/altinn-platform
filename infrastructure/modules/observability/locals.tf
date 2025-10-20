# Determine if existing resources should be reused based on whether names are provided
locals {
  reuse_rg  = var.azurerm_resource_group_obs_name != null && trimspace(var.azurerm_resource_group_obs_name) != ""
  reuse_law = var.log_analytics_workspace_name != null && trimspace(var.log_analytics_workspace_name) != ""
  reuse_ai  = var.app_insights_name != null && trimspace(var.app_insights_name) != ""
  reuse_amw = var.monitor_workspace_name != null && trimspace(var.monitor_workspace_name) != ""
}

# Reuse existing resources if names are provided
data "azurerm_resource_group" "existing" {
  count = local.reuse_rg ? 1 : 0
  name  = var.azurerm_resource_group_obs_name
}

data "azurerm_log_analytics_workspace" "existing" {
  count               = local.reuse_law ? 1 : 0
  name                = var.log_analytics_workspace_name
  resource_group_name = var.azurerm_resource_group_obs_name
}

data "azurerm_application_insights" "existing" {
  count               = local.reuse_ai ? 1 : 0
  name                = var.app_insights_name
  resource_group_name = var.azurerm_resource_group_obs_name
}

data "azurerm_monitor_workspace" "existing" {
  count               = local.reuse_amw ? 1 : 0
  name                = var.monitor_workspace_name
  resource_group_name = var.azurerm_resource_group_obs_name
}
