resource "azurerm_api_management" "admin_test_apim" {
  name                = "${var.name_prefix}-apim"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
  publisher_name      = "Team-Platform"
  publisher_email     = "test-team-platform@ai-dev.no"

  sku_name = "Developer_1"
}