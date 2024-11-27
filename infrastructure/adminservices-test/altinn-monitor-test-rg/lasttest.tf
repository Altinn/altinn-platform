resource "azurerm_resource_group" "lasttest_rg" {
  name     = "lasttest-rg"
  location = "norwayeast"
}

resource "azurerm_monitor_workspace" "lasttest_amw" {
  name                = "lasttest-amw"
  resource_group_name = azurerm_resource_group.lasttest_rg.name
  location            = azurerm_resource_group.lasttest_rg.location
}
