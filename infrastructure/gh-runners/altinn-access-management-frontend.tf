module "gh_runners_altinn-access-management-frontend" {
  source = "../modules/gh-runners"

  resource_group_name           = azurerm_resource_group.gh_runners.name
  repository_name               = "altinn-access-management-frontend"
  private_runners_address_space = "172.17.138.0/24"
  private_runners_prefix        = "am"
  altinn_app_id                 = var.altinn_app_id
  altinn_app_install_id         = var.altinn_app_install_id
  altinn_app_key                = var.altinn_app_key
  host_ip                       = var.host_ip
  runner_image                  = "ghcr.io/altinn/altinn-access-management-frontend/gh-runner:latest"
  runner_cpu                    = "4.0"
  runner_memory                 = "8Gi"
  tags = merge(local.tags, {
    finops_product = "access-management"
    product        = "access-management"
  })
}
