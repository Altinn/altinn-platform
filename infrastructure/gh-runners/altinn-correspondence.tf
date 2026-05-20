module "gh_runners_altinn_correspondence" {
  source = "../modules/gh-runners"

  resource_group_name           = azurerm_resource_group.gh_runners.name
  repository_name               = "altinn-correspondence"
  private_runners_address_space = "172.17.129.0/24"
  private_runners_prefix        = "corr"
  altinn_app_id                 = var.altinn_app_id
  altinn_app_install_id         = var.altinn_app_install_id
  altinn_app_key                = var.altinn_app_key
  host_ip                       = var.host_ip
  tags = merge(local.tags, {
    finops_product = "altinn-correspondence"
    product        = "altinn-correspondence"
  })
}
