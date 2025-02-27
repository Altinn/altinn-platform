module "aks" {
  source             = "../../modules/aks"
  prefix             = "auth"
  environment        = "at22"
  subscription_id    = var.subscription_id
  location           = "norwayeast"
  aks_sku_tier       = "Free"
  kubernetes_version = "1.30"
  vnet_address_space = [
    "10.202.72.0/21",
    "fd0a:7204:c37f:900::/56"
  ]
  subnet_address_prefixes = {
    aks_syspool  = ["fd0a:7204:c37f:901::/64", "10.202.72.0/24"]
    aks_workpool = ["fd0a:7204:c37f:902::/64", "10.202.73.0/24"]
  }
  pool_configs = {
    syspool = {
      vm_size              = "standard_b2s_v2"
      auto_scaling_enabled = "true"
      node_count           = "1"
      min_count            = "1"
      max_count            = "6"
    }
    workpool = {
      vm_size              = "standard_b2s_v2"
      auto_scaling_enabled = "true"
      node_count           = "0"
      min_count            = "0"
      max_count            = "6"
    }
  }
  aks_acrpull_scopes = [
    "/subscriptions/a6e9ee7d-2b65-41e1-adfb-0c8c23515cf9/resourceGroups/acr/providers/Microsoft.ContainerRegistry/registries/altinncr"
  ]
}
