resource "azurerm_resource_group" "aks" {
  name     = var.azurerm_resource_group_aks_name != "" ? var.azurerm_resource_group_aks_name : "${var.prefix}-${var.environment}-aks-rg"
  location = var.location
}
resource "azurerm_resource_group" "monitor" {
  name     = var.azurerm_resource_group_monitor_name != "" ? var.azurerm_resource_group_monitor_name : "${var.prefix}-${var.environment}-monitor-rg"
  location = var.location
}

resource "azurerm_resource_group" "dis" {
  name = var.azurerm_resource_group_dis_name != "" ? var.azurerm_resource_group_dis_name : "${var.prefix}-${var.environment}-dis-main-rg"
  location = var.location
}
