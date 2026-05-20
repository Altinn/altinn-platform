resource "azurerm_resource_group" "gh_runners" {
  name     = "altinn-org-gh-runners"
  location = "norwayeast"
  tags     = local.tags
}

resource "azurerm_role_assignment" "container_apps_managers" {
  for_each             = toset(var.container_apps_managers)
  scope                = azurerm_resource_group.gh_runners.id
  role_definition_name = "Contributor"
  principal_id         = each.value
}
