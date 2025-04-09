resource "azurerm_role_assignment" "altinncr_acrpush" {
  for_each                         = var.acr_push_object_ids
  principal_id                     = each.value
  role_definition_name             = "AcrPush"
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "altinncr_acrpull" {
  for_each                         = var.acr_pull_object_ids
  principal_id                     = each.value
  role_definition_name             = "AcrPull"
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "altinncr_reader" {
  for_each                         = var.acr_pull_object_ids
  principal_id                     = each.value
  role_definition_name             = "Reader"
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}
