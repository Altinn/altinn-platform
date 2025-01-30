resource "azurerm_log_analytics_workspace" "aks" {
  name                = "${var.prefix}-${var.environment}-aks-law"
  resource_group_name = azurerm_resource_group.aks.name
  location            = azurerm_resource_group.aks.location
  retention_in_days   = 30
  identity {
    type = "SystemAssigned"
  }
}

resource "azurerm_monitor_workspace" "aks" {
  name                = "${var.prefix}-${var.environment}-aks-amw"
  resource_group_name = azurerm_resource_group.aks.name
  location            = azurerm_resource_group.aks.location
}
