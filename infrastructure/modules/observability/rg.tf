resource "azurerm_resource_group" "obs" {
  count    = local.reuse_rg ? 0 : 1
  name     = "${var.prefix}-${var.environment}-obs-rg"
  location = var.location
  lifecycle { prevent_destroy = true }
}
