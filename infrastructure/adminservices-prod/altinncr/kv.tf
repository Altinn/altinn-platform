resource "azurerm_key_vault" "kv" {
  name                = var.acrname
  location            = azurerm_resource_group.acr.location
  resource_group_name = azurerm_resource_group.acr.name
  sku_name            = "standard"
  tenant_id           = data.azurerm_client_config.current.tenant_id
}
