resource "azurerm_resource_group" "obs" {
  name     = var.azurerm_resource_group_obs_name != "" ? var.azurerm_resource_group_obs_name : "${var.prefix}-${var.environment}-obs-rg"
  location = var.location
}
