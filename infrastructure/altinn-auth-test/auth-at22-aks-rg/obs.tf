module "observability" {
  source                          = "../../modules/observability"
  depends_on                      = [module.aks]
  prefix                          = "auth"
  environment                     = "at22"
  oidc_issuer_url                 = module.aks.aks_oidc_issuer_url
  azurerm_resource_group_obs_name = module.aks.aks_node_resource_group
}
