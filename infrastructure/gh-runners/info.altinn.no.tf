# GitHub: Altinn/info.altinn.no (product: infoportal)
module "gh_runners_info_altinn_no" {
  source = "../modules/gh-runners"

  resource_group_name           = azurerm_resource_group.gh_runners.name
  repository_name               = "info.altinn.no"
  private_runners_address_space = "172.17.135.0/24"
  private_runners_prefix        = "infoportal"
  altinn_app_id                 = var.altinn_app_id
  altinn_app_install_id         = var.altinn_app_install_id
  altinn_app_key                = var.altinn_app_key
  host_ip                       = var.host_ip

  # Staged rollout of v0.10.0 (adds yq, needed by Altinn/info.altinn.no#695).
  # Remove this override once proven here and the module default is moved up.
  runner_image = "ghcr.io/altinn/altinn-platform/gh-runner:v0.10.0"

  tags = merge(local.tags, {
    finops_product = "infoportal"
    product        = "infoportal"
  })
}
