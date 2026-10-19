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

# GitHub decides per repo whether its OIDC tokens carry the name-based or the immutable
# subject. Read it on every plan so each identity only federates on the form in use. The
# GitHub App needs the Actions (read) repository permission for this endpoint.
data "github_rest_api" "oidc_subject" {
  for_each = local.products_by_repo
  endpoint = "repos/${var.github_org_name}/${each.key}/actions/oidc/customization/sub"

  lifecycle {
    postcondition {
      condition     = self.code == 200
      error_message = "Could not read the OIDC subject format of ${var.github_org_name}/${each.key} (${self.status}). The GitHub App needs the Actions (read) permission on the repo."
    }
  }
}

module "syncroot_github_repo" {
  source   = "../modules/syncroot-github-repo"
  for_each = local.products_by_repo

  github_repo_name        = each.key
  github_org_name         = var.github_org_name
  github_org_id           = data.github_organization.this.id
  github_sub_claim_prefix = jsondecode(data.github_rest_api.oidc_subject[each.key].body).sub_claim_prefix
  github_environments     = toset(flatten([for product in each.value : tolist(var.product_syncroot_source_repos[product].environments)]))
  github_branches         = var.product_syncroot_source_repos[each.value[0]].branches
  subscription_id         = var.subscription_id
  resource_group_name     = azurerm_resource_group.syncroot_pushers.name
  tags                    = local.common-tags
  product_names           = toset(each.value)
}
