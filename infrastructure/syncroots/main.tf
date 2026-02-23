resource "azurerm_resource_group" "syncroot_pushers" {
  name     = "${var.github_org_name}-oidc-syncroot-pushers"
  location = "norwayeast"
}

module "syncroot_github_repo" {
  source   = "../modules/syncroot-github-repo"
  for_each = var.product_syncroot_source_repos

  github_repo_name    = each.value.repo_name
  github_org_name     = var.github_org_name
  github_environments = each.value.environments
  github_refs         = each.value.refs
  subscription_id     = var.subscription_id
  resource_group_name = azurerm_resource_group.syncroot_pushers.name
  tags                = local.common-tags
  product_name        = each.key
}
