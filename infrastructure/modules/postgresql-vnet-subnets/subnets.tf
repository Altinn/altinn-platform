locals {
  subnet_indices = toset(range(16)) # Creates a set of numbers from 0 to 15
}

resource "azurerm_subnet" "postgresql_subnets" {
  for_each                          = toset([for i in local.subnet_indices : tostring(i)])
  address_prefixes                  = [cidrsubnet(var.vnet_address_space, 4, each.value)]
  name                              = "${var.name}-subnet-${each.value}"
  resource_group_name               = var.resource_group_name
  virtual_network_name              = azurerm_virtual_network.postgresql.name
  private_endpoint_network_policies = "Enabled"
  service_endpoints                 = ["Microsoft.Storage"]
  delegation {
    name = "postgresql-delegation"
    service_delegation {
      name    = "Microsoft.DBforPostgreSQL/flexibleServers"
      actions = ["Microsoft.Network/virtualNetworks/subnets/join/action"]
    }
  }
}
