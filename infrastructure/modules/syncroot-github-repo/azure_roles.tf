resource "azurerm_role_assignment" "altinncr_pusher" {
  name                 = uuidv5("6ba7b810-9dad-11d1-80b4-00c04fd430c8", "${var.github_org_name}-${var.github_repo_name}-${var.product_name}-syncroot")
  role_definition_name = "AcrPush"
  principal_id         = azurerm_user_assigned_identity.syncroot_pusher.principal_id
  scope                = "/subscriptions/${var.subscription_id}/resourceGroups/acr/providers/Microsoft.ContainerRegistry/registries/altinncr"
  principal_type       = "ServicePrincipal"
  skip_service_principal_aad_check = true
}