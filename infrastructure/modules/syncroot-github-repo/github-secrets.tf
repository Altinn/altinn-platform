resource "github_actions_secret" "azure_tenant_id" {
  secret_name     = "DIS_SYNCROOT_AZURE_TENANT_ID"
  repository      = var.github_repo_name
  plaintext_value = azurerm_user_assigned_identity.syncroot_pusher.tenant_id
}

resource "github_actions_secret" "azure_subscription_id" {
  secret_name     = "DIS_SYNCROOT_AZURE_SUBSCRIPTION_ID"
  repository      = var.github_repo_name
  plaintext_value = var.subscription_id
}

resource "github_actions_secret" "azure_client_id" {
  secret_name     = "DIS_SYNCROOT_AZURE_CLIENT_ID"
  repository      = var.github_repo_name
  plaintext_value = azurerm_user_assigned_identity.syncroot_pusher.client_id
}
