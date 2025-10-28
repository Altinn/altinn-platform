resource "azurerm_resource_group" "obs" {
  count    = var.azurerm_resource_group_obs_name == null ? 1 : 0
  name     = "${var.prefix}-${var.environment}-obs-rg"
  location = var.location
  lifecycle { prevent_destroy = true }
}
