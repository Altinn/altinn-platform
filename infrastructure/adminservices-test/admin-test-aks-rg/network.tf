resource "azurerm_virtual_network" "vnet" {
  name                = "${var.name_prefix}-vnet"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
  address_space       = var.vnet_address_space
}

resource "azurerm_subnet" "subnets" {
  for_each             = var.subnet_address_prefixes
  name                 = each.key
  resource_group_name  = azurerm_resource_group.rg.name
  virtual_network_name = azurerm_virtual_network.vnet.name
  address_prefixes     = each.value
}

resource "azurerm_public_ip" "pip4" {
  name                 = "${var.name_prefix}-pip4"
  location             = azurerm_resource_group.rg.location
  resource_group_name  = azurerm_kubernetes_cluster.aks.node_resource_group
  allocation_method    = "Static"
  zones                = ["1", "2", "3"]
  ddos_protection_mode = "Enabled"
  sku                  = "Standard"
  ip_version           = "IPv4"
  domain_name_label    = var.name_prefix
}

resource "azurerm_public_ip" "pip6" {
  name                 = "${var.name_prefix}-pip6"
  location             = azurerm_resource_group.rg.location
  resource_group_name  = azurerm_kubernetes_cluster.aks.node_resource_group
  allocation_method    = "Static"
  zones                = ["1", "2", "3"]
  ddos_protection_mode = "Enabled"
  sku                  = "Standard"
  ip_version           = "IPv6"
  domain_name_label    = var.name_prefix
}

resource "azurerm_public_ip_prefix" "prefix4" {
  name                = "${var.name_prefix}-prefix4"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  ip_version          = "IPv4"
  prefix_length       = "31"
  zones               = ["1", "2", "3"]
}

resource "azurerm_public_ip_prefix" "prefix6" {
  name                = "${var.name_prefix}-prefix6"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  ip_version          = "IPv6"
  prefix_length       = "127"
  zones               = ["1", "2", "3"]
}
