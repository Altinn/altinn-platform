resource "azurerm_virtual_network" "postgresql" {
  resource_group_name = var.resource_group_name
  address_space       = [var.vnet_address_space]
  name                = var.name
  location            = var.location
  tags                = var.tags
}

resource "azurerm_virtual_network_peering" "postgresql_to_peered_vnet" {
  depends_on                   = [azurerm_subnet.postgresql_subnets]
  name                         = "${azurerm_virtual_network.postgresql.name}-to-${var.peered_vnets.name}"
  resource_group_name          = var.resource_group_name
  virtual_network_name         = azurerm_virtual_network.postgresql.name
  remote_virtual_network_id    = var.peered_vnets.id
  allow_virtual_network_access = true
  allow_forwarded_traffic      = true
  allow_gateway_transit        = false
  use_remote_gateways          = false
}

resource "azurerm_virtual_network_peering" "peered_vnet_to_postgresql" {
  depends_on                   = [azurerm_subnet.postgresql_subnets]
  name                         = "${var.peered_vnets.name}-to-${azurerm_virtual_network.postgresql.name}"
  resource_group_name          = var.peered_vnets.resource_group_name
  virtual_network_name         = var.peered_vnets.name
  remote_virtual_network_id    = azurerm_virtual_network.postgresql.id
  allow_virtual_network_access = true
  allow_forwarded_traffic      = true
  allow_gateway_transit        = false
  use_remote_gateways          = false
}
