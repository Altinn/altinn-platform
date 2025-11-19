data "azurerm_client_config" "current" {}

locals {
  # hide it from plan / apply since linters can complain
  tenant_id = sensitive(data.azurerm_client_config.current.tenant_id)
}

module "aks" {
  source                  = "../../modules/aks"
  prefix                  = var.team_name
  environment             = var.environment
  subscription_id         = var.subscription_id
  kubernetes_version      = var.kubernetes_version
  vnet_address_space      = var.aks_vnet_address_spaces
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
  depends_on                                   = [module.aks, module.observability, module.dns-child-zone]
  source                                       = "../../modules/aks-resources"
  subscription_id                              = var.subscription_id
  aks_node_resource_group                      = module.aks.aks_node_resource_group
  azurerm_kubernetes_cluster_id                = module.aks.azurerm_kubernetes_cluster_id
  flux_release_tag                             = var.flux_release_tag
  pip4_ip_address                              = module.aks.pip4_ip_address
  pip6_ip_address                              = module.aks.pip6_ip_address
  subnet_address_prefixes                      = var.subnet_address_prefixes
  obs_kv_uri                                   = module.observability.key_vault_uri
  obs_client_id                                = module.observability.obs_client_id
  obs_tenant_id                                = local.tenant_id
  environment                                  = var.environment
  syncroot_namespace                           = var.team_name
  grafana_endpoint                             = module.grafana.grafana_endpoint
  token_grafana_operator                       = module.grafana.token_grafana_operator
  enable_dis_identity_operator                 = true
  enable_grafana_operator                      = true
  enable_cert_manager_tls_issuer               = true
  tls_cert_manager_workload_identity_client_id = module.dns-child-zone.azuread_cert_manager_client_id
  tls_cert_manager_zone_name                   = module.dns-child-zone.azurerm_dns_zone_name
  tls_cert_manager_zone_rg_name                = module.dns-child-zone.azurerm_dns_zone_resource_group_name
  azurerm_dis_identity_resource_group_id       = module.aks.dis_resource_group_id
  azurerm_kubernetes_cluster_oidc_issuer_url   = module.aks.aks_oidc_issuer_url
  lakmus_client_id                             = module.observability.lakmus_client_id
  developer_entra_id_group                     = var.developer_entra_id_group
  linkerd_default_inbound_policy               = "cluster-authenticated"
  grafana_redirect_dns                         = module.dns-child-zone.azurerm_dns_zone_name
}
