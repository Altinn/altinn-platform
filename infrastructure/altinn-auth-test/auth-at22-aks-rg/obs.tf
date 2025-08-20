module "observability" {
  source                        = "../../modules/observability"
  depends_on                    = [module.aks]
  prefix                        = "auth"
  environment                   = "at22"
  enable_aks_monitoring         = true
  azurerm_kubernetes_cluster_id = module.aks.azurerm_kubernetes_cluster_id
  oidc_issuer_url               = module.aks.aks_oidc_issuer_url
}
