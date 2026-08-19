# GitHub: Altinn/altinn-support-private (product: altinn-support)
module "gh_runners_support" {
  source = "../modules/gh-runners"

  resource_group_name           = azurerm_resource_group.gh_runners.name
  repository_name               = "altinn-support-private"
  private_runners_address_space = "172.17.137.0/24"
  private_runners_prefix        = "asupport"
  altinn_app_id                 = var.altinn_app_id
  altinn_app_install_id         = var.altinn_app_install_id
  altinn_app_key                = var.altinn_app_key
  host_ip                       = var.host_ip
  runner_labels                 = "altinn-support-runner"
  tags = merge(local.tags, {
    finops_product = "altinn-support"
    product        = "altinn-support"
  })
}
