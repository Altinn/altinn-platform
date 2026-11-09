module "gh_runners_dis_padawan" {
  source = "../modules/gh-runners"

  resource_group_name           = azurerm_resource_group.gh_runners.name
  repository_name               = "dis-padawan"
  private_runners_address_space = "172.17.139.0/24"
  private_runners_prefix        = "padawan"
  altinn_app_id                 = var.altinn_app_id
  altinn_app_install_id         = var.altinn_app_install_id
  altinn_app_key                = var.altinn_app_key
  host_ip                       = var.host_ip
  runner_cpu                    = "1.0"
  runner_memory                 = "2Gi"
  # Testing the rate limit hardening on dis-padawan before rolling it out to
  # the other repositories: the new runner image and ETags in the scaler.
  runner_image        = "ghcr.io/altinn/altinn-platform/gh-runner:v0.11.0"
  scaler_enable_etags = true
  tags = merge(local.tags, {
    finops_product = "altinn-platform"
    product        = "altinn-platform"
  })
}
