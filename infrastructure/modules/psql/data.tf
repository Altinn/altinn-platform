data "azurerm_client_config" "current" {}

data "azuread_service_principal" "current" {
  object_id = data.azurerm_client_config.current.object_id
}

data "azuread_service_principal" "terraform" {
  object_id = var.terraform_sp_object_id
}

data "azuread_group" "psql_admin_groups" {
  for_each  = toset(var.psql_admin_group_ids)
  object_id = each.value
}

data "azurerm_virtual_network" "psql" {
  name                = var.psql_network_name
  resource_group_name = var.psql_network_resource_group
}
