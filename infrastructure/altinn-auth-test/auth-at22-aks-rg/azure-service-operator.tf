module "azure_service_operator" {
  source                                     = "../../modules/azure-service-operator"
  prefix                                     = local.team_name
  environment                                = local.environment
  azurerm_kubernetes_cluster_oidc_issuer_url = module.aks.aks_oidc_issuer_url
  azurerm_kubernetes_cluster_id              = module.aks.azurerm_kubernetes_cluster_id
  azurerm_subscription_id                    = var.subscription_id
}
