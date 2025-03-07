data "azurerm_container_registry" "altinncr" {
  provider            = azurerm.adminservices-prod
  name                = "altinncr"
  resource_group_name = "acr"
}
