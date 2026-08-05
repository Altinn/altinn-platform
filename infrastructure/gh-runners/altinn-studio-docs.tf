module "gh_runners_altinn_studio_docs" {
  source = "../modules/gh-runners"

  resource_group_name           = azurerm_resource_group.gh_runners.name
  repository_name               = "altinn-studio-docs"
  private_runners_address_space = "172.17.131.0/24"
  private_runners_prefix        = "docs"
  altinn_app_id                 = var.altinn_app_id
  altinn_app_install_id         = var.altinn_app_install_id
  altinn_app_key                = var.altinn_app_key
  host_ip                       = var.host_ip
  tags = merge(local.tags, {
    finops_product = "altinn-docs"
    product        = "altinn-docs"
  })
}
