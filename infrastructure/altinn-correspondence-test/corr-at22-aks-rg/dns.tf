module "dns-child-zone" {
  depends_on = [module.aks]
  source     = "../../modules/dns-child-zone"
  providers = {
    azurerm.parent_zone = azurerm.parent_zone
  }
  prefix               = var.team_name
  environment          = var.environment
  cluster_ipv4_address = module.aks.pip4_ip_address
  cluster_ipv6_address = module.aks.pip6_ip_address
  oidc_issuer_url      = module.aks.aks_oidc_issuer_url
}
