resource "azurerm_resource_group" "gh_runners" {
  name     = "altinn-org-gh-runners"
  location = "norwayeast"
  tags     = local.tags
}
