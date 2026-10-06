data "github_organization" "this" {
  name = var.github_org_name
  # Only the org id is needed; skip the repo and member listings behind this data source.
  summary_only = true
}

resource "azurerm_resource_group" "syncroot_pushers" {
  name     = "DIS_github_${lower(var.github_org_name)}_uami-rg"
  location = "norwayeast"
}

locals {
  products_by_repo = {
    for product, repo in var.product_syncroot_source_repos : repo.repo_name => product...
  }
}

module "syncroot_github_repo" {
  source   = "../modules/syncroot-github-repo"
  for_each = local.products_by_repo

  github_repo_name = each.key
  github_org_name  = var.github_org_name
  github_org_id    = data.github_organization.this.id
  # Validation requires identical federation settings for all products in a repository.
  github_environments = var.product_syncroot_source_repos[each.value[0]].environments
  github_branches     = var.product_syncroot_source_repos[each.value[0]].branches
  subscription_id     = var.subscription_id
  resource_group_name = azurerm_resource_group.syncroot_pushers.name
  tags                = local.common-tags
  product_names       = toset(each.value)
}
