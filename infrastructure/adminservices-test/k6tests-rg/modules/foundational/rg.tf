resource "azurerm_resource_group" "k6tests" {
  name     = "k6tests-rg${var.suffix}"
  location = "norwayeast"
}
