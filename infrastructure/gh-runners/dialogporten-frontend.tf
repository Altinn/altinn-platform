# GitHub: Altinn/dialogporten-frontend (product: altinn-arbeidsflate / Arbeidsflate)
module "gh_runners_dialogporten_frontend" {
  source = "../modules/gh-runners"

  resource_group_name           = azurerm_resource_group.gh_runners.name
  repository_name               = "dialogporten-frontend"
  private_runners_address_space = "172.17.133.0/24"
  private_runners_prefix        = "arbeid"
  altinn_app_id                 = var.altinn_app_id
  altinn_app_install_id         = var.altinn_app_install_id
  altinn_app_key                = var.altinn_app_key
  host_ip                       = var.host_ip
  runner_cpu                    = "4.0"
  runner_memory                 = "8Gi"
  tags = merge(local.tags, {
    finops_product = "altinn-arbeidsflate"
    product        = "altinn-arbeidsflate"
  })
}
