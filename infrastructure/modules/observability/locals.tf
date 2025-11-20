# Reuse flags (null/blank name = create)
locals {
  reuse_rg  = var.azurerm_resource_group_obs_name != null && trimspace(var.azurerm_resource_group_obs_name) != ""
  reuse_amw = var.reuse_monitor_workspace
  reuse_law = var.reuse_log_analytics_workspace
  reuse_ai  = var.reuse_application_insights
}

locals {
  rg = local.reuse_rg ? {
    name     = var.azurerm_resource_group_obs_name
    location = var.location
    } : {
    name     = one(azurerm_resource_group.obs).name
    location = one(azurerm_resource_group.obs).location
  }

  law = local.reuse_law ? {
    name = null
    id   = var.log_analytics_workspace_id
    } : {
    name = one(azurerm_log_analytics_workspace.obs).name
    id   = one(azurerm_log_analytics_workspace.obs).id
  }

  amw = local.reuse_amw ? {
    name = var.monitor_workspace_name
    id   = var.monitor_workspace_id
    } : {
    name = one(azurerm_monitor_workspace.obs).name
    id   = one(azurerm_monitor_workspace.obs).id
  }

  ai = local.reuse_ai ? {
    name              = null
    id                = null
    connection_string = var.app_insights_connection_string
    } : {
    name              = one(azurerm_application_insights.obs).name
    id                = one(azurerm_application_insights.obs).id
    connection_string = one(azurerm_application_insights.obs).connection_string
  }
}
