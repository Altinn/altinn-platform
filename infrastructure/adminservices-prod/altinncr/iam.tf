resource "azurerm_role_assignment" "altinncr_acrpush" {
  for_each                         = var.acr_push_object_ids
  principal_id                     = each.value
  role_definition_name             = "AcrPush"
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}