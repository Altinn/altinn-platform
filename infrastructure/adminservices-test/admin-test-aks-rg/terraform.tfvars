# module.aks
name_prefix        = "admin-test"
flux_release_tag   = "at_ring1"
subscription_id    = "1ce8e9af-c2d6-44e7-9c5e-099a308056fe"
kubernetes_version = "1.33"
vnet_address_space = [
  "10.90.0.0/16",
  "fdac:524d:afaf::/56"
]
subnet_address_prefixes = {
  aks_syspool  = ["fdac:524d:afaf:1::/64", "10.90.1.0/24"]
  aks_workpool = ["fdac:524d:afaf:2::/64", "10.90.2.0/24"]
}
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
grafana_endpoint         = "https://altinn-grafana-test-b2b8dpdkcvfuhfd3.eno.grafana.azure.com"
developer_entra_id_group = "09599a84-645b-4217-853f-01700a17cd4a"
