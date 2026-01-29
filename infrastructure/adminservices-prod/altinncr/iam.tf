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
  principal_id                     = azurerm_user_assigned_identity.github_pusher.principal_id
  role_definition_name             = "AcrPush"
  principal_type                   = "ServicePrincipal"
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "altinncr_user_access_admin_acr_pull" {
  for_each                         = { for assignee in var.user_access_admin_acr_pull_object_ids : assignee.object_id => assignee }
  principal_id                     = each.value.object_id
  role_definition_name             = "User Access Administrator"
  principal_type                   = each.value.type
  scope                            = azurerm_container_registry.acr.id
  skip_service_principal_aad_check = true
  condition                        = "((!(ActionMatches{'Microsoft.Authorization/roleAssignments/write'})) OR (@Request[Microsoft.Authorization/roleAssignments:RoleDefinitionId] ForAnyOfAnyValues:GuidEquals {7f951dda-4ed3-4680-a7ca-43fe172d538d})) AND ((!(ActionMatches{'Microsoft.Authorization/roleAssignments/delete'})) OR (@Resource[Microsoft.Authorization/roleAssignments:RoleDefinitionId] ForAnyOfAnyValues:GuidEquals {7f951dda-4ed3-4680-a7ca-43fe172d538d}))"
  condition_version                = "2.0"
}
