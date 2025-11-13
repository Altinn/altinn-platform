module "observability" {
  source                         = "../../modules/observability"
  depends_on                     = [module.aks]
  prefix                         = var.team_name
  environment                    = var.environment
  enable_aks_monitoring          = true
  azurerm_kubernetes_cluster_id  = module.aks.azurerm_kubernetes_cluster_id
  oidc_issuer_url                = module.aks.aks_oidc_issuer_url
  tenant_id                      = local.tenant_id
  subscription_id                = var.subscription_id
  ci_service_principal_object_id = data.azurerm_client_config.current.object_id
}
