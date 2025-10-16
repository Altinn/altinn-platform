# Determine if existing resources should be reused based on whether names are provided
locals {
  reuse_law = length(trimspace(coalesce(var.log_analytics_workspace_name, ""))) > 0
  reuse_ai  = length(trimspace(coalesce(var.app_insights_name, ""))) > 0
  reuse_amw = length(trimspace(coalesce(var.monitor_workspace_name, ""))) > 0
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
