resource "azurerm_role_assignment" "altinncr_pusher" {
  name                             = uuidv5("6ba7b810-9dad-11d1-80b4-00c04fd430c8", "${var.github_org_name}-${var.github_repo_name}-${var.product_name}-syncroot")
  role_definition_name             = "AcrPush"
  principal_id                     = azurerm_user_assigned_identity.syncroot_pusher.principal_id
  scope                            = "/subscriptions/${var.subscription_id}/resourceGroups/acr/providers/Microsoft.ContainerRegistry/registries/altinncr"
  principal_type                   = "ServicePrincipal"
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "altinncr_repo_writer" {
  name                             = uuidv5("6ba7b810-9dad-11d1-80b4-00c04fd430c8", "${var.github_org_name}-${var.github_repo_name}-${var.product_name}-syncroot")
  role_definition_name             = "Container Registry Repository Writer"
  principal_id                     = azurerm_user_assigned_identity.syncroot_pusher.principal_id
  scope                            = "/subscriptions/${var.subscription_id}/resourceGroups/acr/providers/Microsoft.ContainerRegistry/registries/altinncr"
  principal_type                   = "ServicePrincipal"
  condition_version                = "2.0"
  condition                        = "((!(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/content/write'}) AND !(ActionMatches{'Microsoft.ContainerRegistry/registries/repositories/metadata/write'})) OR (@Request[Microsoft.ContainerRegistry/registries/repositories:name] StringStartsWith '${var.product_name}/'))"
  skip_service_principal_aad_check = true
}
