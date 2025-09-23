data "azurerm_client_config" "current" {}

data "azuread_service_principal" "current" {
  object_id = data.azurerm_client_config.current.object_id
}

data "azuread_group" "psql_admin_groups" {
  for_each = toset(var.psql_AdminGroups)
  display_name = each.value
}

data "azurerm_log_analytics_workspace" "workspace" {
  name                = var.log_analytics_workspace_name
  resource_group_name = var.log_analytics_workspace_rg
}

data "azurerm_virtual_network" "psql" {
  name                = var.psql_NetworkName
  resource_group_name = var.psql_NetworkResourceGroup
}