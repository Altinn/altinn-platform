resource "azurerm_virtual_network" "aks" {
  name                = var.azurerm_virtual_network_aks_name != "" ? var.azurerm_virtual_network_aks_name : "${var.prefix}-${var.environment}-aks-vnet"
  location            = azurerm_resource_group.aks.location
  resource_group_name = azurerm_resource_group.aks.name
  address_space       = var.vnet_address_space
}

resource "azurerm_subnet" "aks" {
  for_each             = var.subnet_address_prefixes
  name                 = each.key
  resource_group_name  = azurerm_resource_group.aks.name
  virtual_network_name = azurerm_virtual_network.aks.name
  address_prefixes     = each.value
}

resource "azurerm_public_ip" "pip4" {
  name                = var.azurerm_virtual_public_ip_pip4_name != "" ? var.azurerm_virtual_public_ip_pip4_name : "${var.prefix}-${var.environment}-aks-pip4"
  location            = azurerm_resource_group.aks.location
  resource_group_name = azurerm_kubernetes_cluster.aks.node_resource_group
  allocation_method   = "Static"
  zones               = ["1", "2", "3"]
  sku                 = "Standard"
  ip_version          = "IPv4"
  domain_name_label   = "${var.prefix}-${var.environment}"
}

resource "azurerm_public_ip" "pip6" {
  name                = var.azurerm_virtual_public_ip_pip6_name != "" ? var.azurerm_virtual_public_ip_pip6_name : "${var.prefix}-${var.environment}-aks-pip6"
  location            = azurerm_resource_group.aks.location
  resource_group_name = azurerm_kubernetes_cluster.aks.node_resource_group
  allocation_method   = "Static"
  zones               = ["1", "2", "3"]
  sku                 = "Standard"
  ip_version          = "IPv6"
  domain_name_label   = "${var.prefix}-${var.environment}"
}

resource "azurerm_public_ip_prefix" "prefix4" {
  name                = var.azurerm_public_ip_prefix_prefix4_name != "" ? var.azurerm_public_ip_prefix_prefix4_name : "${var.prefix}-${var.environment}-aks-prefix4"
  resource_group_name = azurerm_resource_group.aks.name
  location            = azurerm_resource_group.aks.location
  ip_version          = "IPv4"
  prefix_length       = "31"
  zones               = ["1", "2", "3"]
}

resource "azurerm_public_ip_prefix" "prefix6" {
  name                = var.azurerm_public_ip_prefix_prefix6_name != "" ? var.azurerm_public_ip_prefix_prefix6_name : "${var.prefix}-${var.environment}-aks-prefix6"
  resource_group_name = azurerm_resource_group.aks.name
  location            = azurerm_resource_group.aks.location
  ip_version          = "IPv6"
  prefix_length       = "127"
  zones               = ["1", "2", "3"]
}

# Assign "Network Contributor" Role to AKS Managed Identity
resource "azurerm_role_assignment" "network_contributor" {
  scope                            = azurerm_resource_group.aks.id
  role_definition_name             = "Network Contributor"
  principal_id                     = azurerm_kubernetes_cluster.aks.identity[0].principal_id
  skip_service_principal_aad_check = true
}
