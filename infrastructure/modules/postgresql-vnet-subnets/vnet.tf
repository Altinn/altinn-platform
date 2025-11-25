resource "azurerm_virtual_network" "postgresql" {
  resource_group_name = var.resource_group_name
  address_space       = [var.vnet_address_space]
  name                = var.name
  location            = var.location
  tags                = var.tags
}
