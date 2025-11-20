module "azure_service_operator" {
  depends_on                                 = [module.aks, module.aks_resources]
  source                                     = "../../modules/azure-service-operator"
  prefix                                     = local.team_name
  environment                                = local.environment
  azurerm_kubernetes_cluster_oidc_issuer_url = module.aks.aks_oidc_issuer_url
  azurerm_kubernetes_cluster_id              = module.aks.azurerm_kubernetes_cluster_id
  azurerm_subscription_id                    = var.subscription_id
  dis_resource_group_id                      = module.aks.dis_resource_group_id
  flux_release_tag                           = var.flux_release_tag
}
