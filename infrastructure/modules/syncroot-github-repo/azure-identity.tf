resource "azurerm_user_assigned_identity" "syncroot_pusher" {
  name                = "${var.github_org_name}-${replace(var.github_repo_name, ".", "_")}-${var.product_name}-syncroot"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags = merge(var.tags, {
    submodule = "oidc-syncroot-pusher"
    product   = var.product_name
  })
}

resource "azurerm_federated_identity_credential" "syncroot_pusher_envs" {
  for_each            = var.github_environments
  name                = "${var.github_org_name}-${replace(var.github_repo_name, ".", "_")}-env-${each.value}"
  resource_group_name = var.resource_group_name
  parent_id           = azurerm_user_assigned_identity.syncroot_pusher.id
  issuer              = "https://token.actions.githubusercontent.com"
  audience            = ["api://AzureADTokenExchange"]
  subject             = "repo:${var.github_org_name}/${var.github_repo_name}:environment:${each.value}"
}

resource "azurerm_federated_identity_credential" "syncroot_pusher_branches" {
  for_each            = var.github_branches
  name                = "${var.github_org_name}-${replace(var.github_repo_name, ".", "_")}-ref-${each.value}"
  resource_group_name = var.resource_group_name
  parent_id           = azurerm_user_assigned_identity.syncroot_pusher.id
  issuer              = "https://token.actions.githubusercontent.com"
  audience            = ["api://AzureADTokenExchange"]
  subject             = "repo:${var.github_org_name}/${var.github_repo_name}:ref:refs/heads/${each.value}"
}
