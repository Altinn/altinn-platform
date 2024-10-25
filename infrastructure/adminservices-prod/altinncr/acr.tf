resource "azurerm_resource_group" "acr" {
  location = "norwayeast"
  name     = "acr"
}
resource "azurerm_container_registry" "acr" {
  name                = var.acrname
  resource_group_name = azurerm_resource_group.acr.name
  location            = azurerm_resource_group.acr.location
  sku                 = "Standard"
}

resource "azurerm_container_registry_cache_rule" "cache_rule" {
  for_each              = { for rule in var.cache_rules : rule.name => rule }
  name                  = each.value.name
  container_registry_id = azurerm_container_registry.acr.id
  target_repo           = each.value.target_repo
  source_repo           = each.value.source_repo
  credential_set_id     = each.value.credential_set_id != null ? "${azurerm_container_registry.acr.id}${each.value.credential_set_id}" : null
}
