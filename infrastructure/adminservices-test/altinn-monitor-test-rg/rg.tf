resource "azurerm_resource_group" "rg" {
  name     = "altinn-monitor-test-rg"
  location = "norwayeast"
}

resource "azurerm_role_assignment" "altinn_apps_terraform" {
  for_each                         = toset(["Contributor", "User Access Administrator"])
  scope                            = azurerm_resource_group.rg.id
  role_definition_name             = each.key
  principal_id                     = "26e1eede-69a5-46b2-abea-f839092273ba"
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "platform_terraform" {
  for_each                         = toset(["Contributor", "User Access Administrator"])
  scope                            = azurerm_resource_group.rg.id
  role_definition_name             = each.key
  principal_id                     = "641fc568-3e2f-4174-a7ce-d91f50c8e6d6"
  skip_service_principal_aad_check = true
}

locals {
  parsed_endpoint_id = provider::azurerm::parse_resource_id("${azurerm_monitor_workspace.altinn_monitor.default_data_collection_endpoint_id}")
}

resource "azurerm_role_assignment" "altinn_apps_terraform_ma_rg" {
  for_each                         = toset(["Contributor", "User Access Administrator"])
  scope                            = "${data.azurerm_subscription.current.id}/resourceGroups/${local.parsed_endpoint_id["resource_group_name"]}"
  role_definition_name             = each.key
  principal_id                     = "26e1eede-69a5-46b2-abea-f839092273ba"
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "platform_terraform_ma_rg" {
  for_each                         = toset(["Contributor", "User Access Administrator"])
  scope                            = "${data.azurerm_subscription.current.id}/resourceGroups/${local.parsed_endpoint_id["resource_group_name"]}"
  role_definition_name             = each.key
  principal_id                     = "641fc568-3e2f-4174-a7ce-d91f50c8e6d6"
  skip_service_principal_aad_check = true
}
