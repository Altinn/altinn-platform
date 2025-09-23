resource "azurerm_role_assignment" "apim_service_contributor" {
  for_each             = var.apim_service_contributors
  scope                = azurerm_api_management.apim.id
  role_definition_name = "API Management Service Contributor"
  principal_id         = each.value
}
