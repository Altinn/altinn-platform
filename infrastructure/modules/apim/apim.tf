resource "azurerm_api_management" "apim" {
  name                = "${var.prefix}-${var.environment}-${random_string.apim_random_part.result}-apim"
  location            = azurerm_resource_group.apim_rg.location
  resource_group_name = azurerm_resource_group.apim_rg.name
  publisher_name      = var.publisher
  publisher_email     = var.publisher_email
  sku_name            = var.sku_name
  tags                = var.tags
}
