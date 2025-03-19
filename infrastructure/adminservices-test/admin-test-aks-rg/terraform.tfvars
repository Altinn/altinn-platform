subscription_id                     = "1ce8e9af-c2d6-44e7-9c5e-099a308056fe"
admin_services_prod_subscription_id = "a6e9ee7d-2b65-41e1-adfb-0c8c23515cf9"
name_prefix                         = "admin-test"
vnet_address_space                  = ["10.90.0.0/16", "fdac:524d:afaf::/56"]
subnet_address_prefixes = {
  aks_syspool  = ["fdac:524d:afaf:1::/64", "10.90.1.0/24"]
  aks_workpool = ["fdac:524d:afaf:2::/64", "10.90.2.0/24"]
}
pool_configs = {
  syspool = {
    vm_size   = "standard_b2s_v2"
    min_count = "1"
    max_count = "3"
  }
  workpool = {
    vm_size   = "standard_b2s_v2"
    min_count = "0"
    max_count = "6"
  }
}
kubernetes_version   = "1.30"
aks_sku_tier         = "Free"
flux_release_tag     = "latest"
cert_manager_version = "1.17.x"
