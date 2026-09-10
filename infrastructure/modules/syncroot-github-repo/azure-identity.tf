resource "azurerm_user_assigned_identity" "syncroot_pusher" {
  name                = "${var.github_org_name}-${replace(var.github_repo_name, ".", "_")}-${var.product_name}-syncroot"
  location            = var.location
  resource_group_name = var.resource_group_name
  tags = merge(var.tags, {
    submodule = "oidc-syncroot-pusher"
    product   = var.product_name
  })
}

data "github_repository" "syncroot_source" {
  full_name = "${var.github_org_name}/${var.github_repo_name}"
}

locals {
  credential_name_prefix = "${var.github_org_name}-${replace(var.github_repo_name, ".", "_")}"

  # Repos created, renamed or transferred after 2026-07-15 get OIDC subject claims that
  # embed the immutable org and repo ids instead of the mutable names, and repos can opt
  # in ahead of that. Entra compares subjects verbatim, so federate on both forms and let
  # GitHub decide which one the token carries. The name-based credentials can go once
  # every repo below has flipped.
  # https://github.blog/changelog/2026-04-23-immutable-subject-claims-for-github-actions-oidc-tokens/
  subject_repo           = "repo:${var.github_org_name}/${var.github_repo_name}"
  subject_repo_immutable = "repo:${var.github_org_name}@${var.github_org_id}/${var.github_repo_name}@${data.github_repository.syncroot_source.repo_id}"
}

resource "azurerm_federated_identity_credential" "syncroot_pusher_envs" {
  for_each                  = var.github_environments
  name                      = "${local.credential_name_prefix}-env-${each.value}"
  user_assigned_identity_id = azurerm_user_assigned_identity.syncroot_pusher.id
  issuer                    = "https://token.actions.githubusercontent.com"
  audience                  = ["api://AzureADTokenExchange"]
  subject                   = "${local.subject_repo}:environment:${each.value}"
}

resource "azurerm_federated_identity_credential" "syncroot_pusher_envs_immutable" {
  for_each                  = var.github_environments
  name                      = "${local.credential_name_prefix}-env-${each.value}-immutable"
  user_assigned_identity_id = azurerm_user_assigned_identity.syncroot_pusher.id
  issuer                    = "https://token.actions.githubusercontent.com"
  audience                  = ["api://AzureADTokenExchange"]
  subject                   = "${local.subject_repo_immutable}:environment:${each.value}"
}

resource "azurerm_federated_identity_credential" "syncroot_pusher_branches" {
  for_each                  = var.github_branches
  name                      = "${local.credential_name_prefix}-ref-${each.value}"
  user_assigned_identity_id = azurerm_user_assigned_identity.syncroot_pusher.id
  issuer                    = "https://token.actions.githubusercontent.com"
  audience                  = ["api://AzureADTokenExchange"]
  subject                   = "${local.subject_repo}:ref:refs/heads/${each.value}"
}

resource "azurerm_federated_identity_credential" "syncroot_pusher_branches_immutable" {
  for_each                  = var.github_branches
  name                      = "${local.credential_name_prefix}-ref-${each.value}-immutable"
  user_assigned_identity_id = azurerm_user_assigned_identity.syncroot_pusher.id
  issuer                    = "https://token.actions.githubusercontent.com"
  audience                  = ["api://AzureADTokenExchange"]
  subject                   = "${local.subject_repo_immutable}:ref:refs/heads/${each.value}"
}
