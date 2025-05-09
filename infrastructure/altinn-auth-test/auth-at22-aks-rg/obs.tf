module "observability" {
  source                          = "../../modules/observability"
  depends_on                      = [module.aks]
  prefix                          = "auth"
  environment                     = "at22"
  location                        = var.location
  azurerm_resource_group_obs_name = module.aks.aks_node_resource_group
  kube_context                    = module.aks.aks_name
  kubeconfig_path                 = "~/.kube/config"
}