# Resource Group (create if not provided)
resource "azurerm_resource_group" "obs" {
  count    = local.reuse_rg ? 0 : 1
  name     = "${var.prefix}-${var.environment}-obs-rg"
  location = var.location
  tags     = var.tags
}

# Log Analytics Workspace (create only if not reusing)
resource "azurerm_log_analytics_workspace" "obs" {
  count               = local.reuse_law ? 0 : 1
  name                = "${var.prefix}-${var.environment}-obs-law"
  resource_group_name = local.rg.name
  location            = local.rg.location
  retention_in_days   = var.log_analytics_retention_days
  lifecycle { prevent_destroy = true }
  tags = var.tags
}

# Azure Monitor Workspace (create only if not reusing)
resource "azurerm_monitor_workspace" "obs" {
  count               = local.reuse_amw ? 0 : 1
  name                = "${var.prefix}-${var.environment}-obs-amw"
  resource_group_name = local.rg.name
  location            = local.rg.location
  lifecycle { prevent_destroy = true }
  tags = var.tags
}

# Application Insights (create only if not reusing)
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
