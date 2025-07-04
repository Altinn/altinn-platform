data "azurerm_client_config" "current" {}

locals {
  # hide it from plan / apply since linters can complain
  tenant_id        = sensitive(data.azurerm_client_config.current.tenant_id)
  team_name        = "auth"
  environment      = "at22"
  flux_release_tag = "at_ring2"
}

module "aks" {
  source             = "../../modules/aks"
  prefix             = local.team_name
  environment        = local.environment
  subscription_id    = var.subscription_id
  kubernetes_version = "1.32"
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
  depends_on                                 = [module.aks, module.observability, module.azure_service_operator]
  source                                     = "../../modules/aks-resources"
  aks_node_resource_group                    = module.aks.aks_node_resource_group
  azurerm_kubernetes_cluster_id              = module.aks.azurerm_kubernetes_cluster_id
  flux_release_tag                           = local.flux_release_tag
  pip4_ip_address                            = module.aks.pip4_ip_address
  pip6_ip_address                            = module.aks.pip6_ip_address
  subnet_address_prefixes                    = var.subnet_address_prefixes
  obs_kv_uri                                 = module.observability.key_vault_uri
  obs_client_id                              = module.observability.obs_client_id
  obs_tenant_id                              = local.tenant_id
  environment                                = local.environment
  syncroot_namespace                         = local.team_name
  grafana_endpoint                           = module.grafana.grafana_endpoint
  token_grafana_operator                     = module.grafana.token_grafana_operator
  enable_dis_identity_operator               = true
  azurerm_dis_identity_resource_group_id     = module.aks.dis_resource_group_id
  azurerm_kubernetes_cluster_oidc_issuer_url = module.aks.aks_oidc_issuer_url
}
