resource "azurerm_log_analytics_workspace" "obs" {
  count               = var.log_analytics_workspace_name == null ? 1 : 0
  name                = "${var.prefix}-${var.environment}-obs-law"
  resource_group_name = try(azurerm_resource_group.obs[0].name, var.azurerm_resource_group_obs_name)
  location            = var.location
  retention_in_days   = var.log_analytics_retention_days
  lifecycle { prevent_destroy = true }
  tags = var.tags
}

resource "azurerm_monitor_workspace" "obs" {
  count               = var.monitor_workspace_name == null ? 1 : 0
  name                = "${var.prefix}-${var.environment}-obs-amw"
  resource_group_name = try(azurerm_resource_group.obs[0].name, var.azurerm_resource_group_obs_name)
  location            = var.location
  lifecycle { prevent_destroy = true }
  tags = var.tags
}

resource "azurerm_application_insights" "obs" {
  count               = var.app_insights_name == null ? 1 : 0
  name                = "${var.prefix}-${var.environment}-obs-appinsights"
  resource_group_name = try(azurerm_resource_group.obs[0].name, var.azurerm_resource_group_obs_name)
  location            = var.location
  workspace_id        = try(azurerm_log_analytics_workspace.obs[0].id, data.azurerm_log_analytics_workspace.existing[0].id)
  application_type    = var.app_insights_app_type
  retention_in_days   = 30
  lifecycle { prevent_destroy = true }
  tags = var.tags
}
