data "github_organization" "this" {
  name = var.github_org_name
  # Only the org id is needed; skip the repo and member listings behind this data source.
  summary_only = true
}

resource "azurerm_resource_group" "syncroot_pushers" {
  name     = "DIS_github_${lower(var.github_org_name)}_uami-rg"
  location = "norwayeast"
}

module "syncroot_github_repo" {
  source   = "../modules/syncroot-github-repo"
  for_each = var.product_syncroot_source_repos

  github_repo_name    = each.value.repo_name
  github_org_name     = var.github_org_name
  github_org_id       = data.github_organization.this.id
  github_environments = each.value.environments
  github_branches     = each.value.branches
  subscription_id     = var.subscription_id
  resource_group_name = azurerm_resource_group.syncroot_pushers.name
  tags                = local.common-tags
  product_name        = each.key
}
