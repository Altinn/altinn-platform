resource "azurerm_role_assignment" "altinncr_acrpush" {
  for_each                         = { for pusher in var.acr_push_object_ids : pusher.object_id => pusher }
  principal_id                     = each.value.object_id
  role_definition_name             = "AcrPush"
  principal_type                   = each.value.type
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "altinncr_acrpull" {
  for_each                         = { for puller in var.acr_pull_object_ids : puller.object_id => puller }
  principal_id                     = each.value.object_id
  role_definition_name             = "AcrPull"
  principal_type                   = each.value.type
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "altinncr_reader" {
  for_each                         = { for puller in var.acr_pull_object_ids : puller.object_id => puller }
  principal_id                     = each.value.object_id
  role_definition_name             = "Reader"
  principal_type                   = each.value.type
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "altinncr_acrpush_altinn_platform" {
  principal_id                     = azurerm_user_assigned_identity.github-pusher.principal_id
  role_definition_name             = "AcrPush"
  principal_type                   = "ServicePrincipal"
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}