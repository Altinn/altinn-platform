resource "azurerm_monitor_workspace" "k6tests_amw" {
  name                = "k6tests-amw"
  resource_group_name = azurerm_resource_group.k6tests_rg.name
  location            = azurerm_resource_group.k6tests_rg.location
}
