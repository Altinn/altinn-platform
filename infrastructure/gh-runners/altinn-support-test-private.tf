# GitHub: Altinn/altinn-support-test-private (product: altinn-support)
module "gh_runners_support_test" {
  source = "../modules/gh-runners"

  resource_group_name           = azurerm_resource_group.gh_runners.name
  repository_name               = "altinn-support-test-private"
  private_runners_address_space = "172.17.136.0/24"
  private_runners_prefix        = "as-test"
  altinn_app_id                 = var.altinn_app_id
  altinn_app_install_id         = var.altinn_app_install_id
  altinn_app_key                = var.altinn_app_key
  host_ip                       = var.host_ip
  runner_labels                 = "altinn-support-test-runner"
  runner_image                  = "ghcr.io/altinn/altinn-support-private/gh-runner-image:latest"
  tags = merge(local.tags, {
    finops_product = "altinn-support"
    product        = "altinn-support"
  })
}
