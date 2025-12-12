module "azure_service_operator" {
  depends_on                                 = [module.aks, module.infra-resources]
  source                                     = "../../modules/azure-service-operator"
  prefix                                     = var.team_name
  environment                                = var.environment
  azurerm_kubernetes_cluster_oidc_issuer_url = module.aks.aks_oidc_issuer_url
  azurerm_kubernetes_cluster_id              = module.aks.azurerm_kubernetes_cluster_id
  azurerm_kubernetes_workpool_vnet_id        = module.aks.aks_workpool_vnet_id
  azurerm_subscription_id                    = var.subscription_id
  dis_resource_group_id                      = module.aks.dis_resource_group_id
  flux_release_tag                           = var.flux_release_tag
}
