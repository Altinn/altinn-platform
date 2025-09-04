data "azurerm_client_config" "current" {}

locals {
  # hide it from plan / apply since linters can complain
  tenant_id   = sensitive(data.azurerm_client_config.current.tenant_id)
  team_name   = "admin"
  environment = "test"
}

module "aks" {
  source                  = "../../modules/aks"
  prefix                  = local.team_name
  environment             = local.environment
  subscription_id         = var.subscription_id
  kubernetes_version      = var.kubernetes_version
  vnet_address_space      = var.vnet_address_space
  subnet_address_prefixes = var.subnet_address_prefixes
  pool_configs            = var.pool_configs
  aks_acrpull_scopes      = var.aks_acrpull_scopes
  admin_group_object_ids  = var.admin_group_object_ids
  # overrides
  azurerm_resource_group_aks_name               = "admin-test-rg"
  azurerm_virtual_network_aks_name              = "admin-test-vnet"
  azurerm_virtual_public_ip_pip4_name           = "admin-test-pip4"
  azurerm_virtual_public_ip_pip6_name           = "admin-test-pip6"
  azurerm_public_ip_prefix_prefix4_name         = "admin-test-prefix4"
  azurerm_public_ip_prefix_prefix6_name         = "admin-test-prefix6"
  azurerm_kubernetes_cluster_aks_dns_service_ip = "10.250.0.53"
  azurerm_kubernetes_cluster_aks_service_cidrs = [
    "10.250.0.0/16",
    "fd10:59f0:8c79:250::/108"
  ]
}

module "aks_resources" {
  depends_on                       = [module.aks, module.observability]
  source                           = "../../modules/aks-resources"
  subscription_id                  = var.subscription_id
  aks_node_resource_group          = module.aks.aks_node_resource_group
  azurerm_kubernetes_cluster_id    = module.aks.azurerm_kubernetes_cluster_id
  flux_release_tag                 = var.flux_release_tag
  pip4_ip_address                  = module.aks.pip4_ip_address
  pip6_ip_address                  = module.aks.pip6_ip_address
  subnet_address_prefixes          = var.subnet_address_prefixes
  obs_kv_uri                       = module.observability.key_vault_uri
  obs_client_id                    = module.observability.obs_client_id
  obs_tenant_id                    = local.tenant_id
  environment                      = local.environment
  syncroot_namespace               = local.team_name
  grafana_endpoint                 = var.grafana_endpoint
  token_grafana_operator           = var.token_grafana_operator
  grafana_dashboard_release_branch = "main"
  enable_grafana_operator          = true
  lakmus_client_id                 = module.observability.lakmus_client_id
}
