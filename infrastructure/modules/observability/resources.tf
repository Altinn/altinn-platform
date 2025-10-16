resource "azurerm_log_analytics_workspace" "obs" {
  count               = local.reuse_law ? 0 : 1
  name                = "${var.prefix}-${var.environment}-obs-law"
  resource_group_name = local.rg.name
  location            = local.rg.location
  retention_in_days   = var.log_analytics_retention_days
  lifecycle { prevent_destroy = true }
  tags = var.tags
}

resource "azurerm_monitor_workspace" "obs" {
  count               = local.reuse_amw ? 0 : 1
  name                = "${var.prefix}-${var.environment}-obs-amw"
  resource_group_name = local.rg.name
  location            = local.rg.location
  lifecycle { prevent_destroy = true }
  tags = var.tags
}

resource "azurerm_application_insights" "obs" {
  count               = local.reuse_ai ? 0 : 1
  name                = "${var.prefix}-${var.environment}-obs-appinsights"
  resource_group_name = local.rg.name
  location            = local.rg.location
  workspace_id        = local.law.id
  application_type    = var.app_insights_app_type
  retention_in_days   = 30
  lifecycle { prevent_destroy = true }
  tags = var.tags
}

# local values to simplify access to either existing or created resources
locals {

  rg = local.reuse_rg ? {
    name     = one(data.azurerm_resource_group.existing).name
    location = one(data.azurerm_resource_group.existing).location
    id       = one(data.azurerm_resource_group.existing).id
    } : {
    name     = one(azurerm_resource_group.obs).name
    location = one(azurerm_resource_group.obs).location
    id       = one(azurerm_resource_group.obs).id
  }

  law = local.reuse_law ? {
    id   = one(data.azurerm_log_analytics_workspace.existing).id
    name = one(data.azurerm_log_analytics_workspace.existing).name
    } : {
    id   = one(azurerm_log_analytics_workspace.obs).id
    name = one(azurerm_log_analytics_workspace.obs).name
  }

  ai = local.reuse_ai ? {
    id                = one(data.azurerm_application_insights.existing).id
    connection_string = one(data.azurerm_application_insights.existing).connection_string
    } : {
    id                = one(azurerm_application_insights.obs).id
    connection_string = one(azurerm_application_insights.obs).connection_string
  }

  amw = local.reuse_amw ? {
    id   = one(data.azurerm_monitor_workspace.existing).id
    name = one(data.azurerm_monitor_workspace.existing).name
    } : {
    id   = one(azurerm_monitor_workspace.obs).id
    name = one(azurerm_monitor_workspace.obs).name
  }
}
