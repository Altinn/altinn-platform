import {
  to = azurerm_resource_group.gh_runners
  id = "/subscriptions/d43d5057-8389-40d5-88c4-04db9275cbf2/resourceGroups/altinn-org-gh-runners"
}

resource "azurerm_resource_group" "gh_runners" {
  name     = "altinn-org-gh-runners"
  location = "norwayeast"
  tags     = local.tags
}

resource "azurerm_role_assignment" "terraform_reader_kv_secrets" {
  scope                = azurerm_resource_group.gh_runners.id
  role_definition_name = "Key Vault Secrets User"
  principal_id         = var.terraform_reader_principal_id
}

resource "azurerm_role_assignment" "terraform_reader_kv_contributor" {
  scope                = azurerm_resource_group.gh_runners.id
  role_definition_name = "Key Vault Contributor"
  principal_id         = var.terraform_reader_principal_id
}

resource "azurerm_role_assignment" "container_apps_managers" {
  for_each             = toset(var.container_apps_managers)
  scope                = azurerm_resource_group.gh_runners.id
  role_definition_name = "Contributor"
  principal_id         = each.value
}
