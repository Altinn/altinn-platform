subscription_id = "cc6b5886-36af-4d54-9d30-1b0fffc736d8"
aks_vnet_address_spaces = [
  "10.203.0.0/20",
  "fd96:20bf:3235::/56"
]
subnet_address_prefixes = {
  aks_syspool  = ["fd96:20bf:3235:1::/64", "10.203.1.0/24"]
  aks_workpool = ["fd96:20bf:3235:2::/64", "10.203.2.0/24"]
}
kubernetes_version       = "1.33"
team_name                = "corr"
environment              = "at22"
flux_release_tag         = "at_ring2"
developer_entra_id_group = "12b73376-8726-493c-8d27-aa87e5213e6b" # Altinn-30-Correspondence-Test-Developers
