module "dis_apim_test" {
  source                             = "../../modules/dis-apim-operator"
  apim_id                            = "/subscriptions/1ce8e9af-c2d6-44e7-9c5e-099a308056fe/resourceGroups/altinn-apim-test-rg/providers/Microsoft.ApiManagement/service/altinn-apim-test-apim"
  kubernetes_cluster_id              = module.aks.azurerm_kubernetes_cluster_id
  kubernetes_cluster_oidc_issuer_url = module.aks.aks_oidc_issuer_url
  kubernetes_node_location           = "norwayeast"
  kubernetes_node_resource_group     = module.aks.aks_node_resource_group
  apim_subscription_id               = var.subscription_id
  apim_resource_group_name           = "altinn-apim-test-rg"
  apim_service_name                  = "altinn-apim-test-apim"
  target_namespace                   = "dis-altinn-apim-test-operator"
}