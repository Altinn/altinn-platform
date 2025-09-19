resource "azurerm_resource_group" "apim_rg" {
  name     = var.apim_rg_name != "" ? var.apim_rg_name : "${var.prefix}-${var.environment}-apim-rg"
  location = var.location
  tags     = var.tags
}
