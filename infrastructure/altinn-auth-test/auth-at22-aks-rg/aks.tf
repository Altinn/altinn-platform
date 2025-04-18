module "aks" {
  source             = "../../modules/aks"
  prefix             = "auth"
  environment        = "at22"
  subscription_id    = var.subscription_id
  kubernetes_version = "1.30"
  vnet_address_space = [
    "10.202.72.0/21",
    "fd0a:7204:c37f:900::/56"
  ]
  subnet_address_prefixes = var.subnet_address_prefixes
  pool_configs = {
    syspool = {
      vm_size              = "standard_b2s_v2"
      auto_scaling_enabled = true
      node_count           = 1
      min_count            = 1
      max_count            = 6
    }
    workpool = {
      vm_size              = "standard_b2s_v2"
      auto_scaling_enabled = true
      node_count           = 0
      min_count            = 0
      max_count            = 6
    }
  }
  aks_acrpull_scopes = [
    "/subscriptions/a6e9ee7d-2b65-41e1-adfb-0c8c23515cf9/resourceGroups/acr/providers/Microsoft.ContainerRegistry/registries/altinncr"
  ]
  admin_group_object_ids = [
    "09599a84-645b-4217-853f-01700a17cd4a"
  ]
}

module "infra-resources" {
  depends_on                    = [module.aks]
  source                        = "../../modules/aks-resources"
  aks_node_resource_group       = module.aks.aks_node_resource_group
  azurerm_kubernetes_cluster_id = module.aks.azurerm_kubernetes_cluster_id
  flux_release_tag              = "at_ring2"
  pip4_ip_address               = module.aks.pip4_ip_address
  pip6_ip_address               = module.aks.pip6_ip_address
  subnet_address_prefixes       = var.subnet_address_prefixes
}
