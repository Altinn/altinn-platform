module "observability" {
  source                        = "../../modules/observability"
  depends_on                    = [module.aks]
  prefix                        = local.team_name
  environment                   = local.environment
  azurerm_kubernetes_cluster_id = module.aks.azurerm_kubernetes_cluster_id
  oidc_issuer_url               = module.aks.aks_oidc_issuer_url
}
