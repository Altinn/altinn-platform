resource "azurerm_log_analytics_workspace" "obs" {
  count               = local.reuse_law ? 0 : 1
  name                = "${var.prefix}-${var.environment}-obs-law"
  resource_group_name = azurerm_resource_group.obs.name
  location            = var.location
  retention_in_days   = var.log_analytics_retention_days

  tags = var.tags
}

resource "azurerm_monitor_workspace" "obs" {
  count               = local.reuse_amw ? 0 : 1
  name                = "${var.prefix}-${var.environment}-obs-amw"
  resource_group_name = azurerm_resource_group.obs.name
  location            = var.location

  tags = var.tags
}

resource "azurerm_application_insights" "obs" {
  count               = local.reuse_ai ? 0 : 1
  name                = "${var.prefix}-${var.environment}-obs-appinsights"
  resource_group_name = azurerm_resource_group.obs.name
  location            = var.location
  workspace_id        = azurerm_log_analytics_workspace.obs.id
  application_type    = var.app_insights_app_type
  retention_in_days   = 30

  tags = var.tags
}

# local values to simplify access to either existing or created resources
locals {

  law_id = local.reuse_law ? one(data.azurerm_log_analytics_workspace.existing).id : one(azurerm_log_analytics_workspace.obs).id

  ai_connection_string = local.reuse_ai ? one(data.azurerm_application_insights.existing).connection_string : one(azurerm_application_insights.obs).connection_string

  amw_id = local.reuse_amw ? one(data.azurerm_monitor_workspace.existing).id : one(azurerm_monitor_workspace.obs).id
}