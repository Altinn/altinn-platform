subscription_id = "37bac63a-b964-46b2-8de8-ba93c432ea1f"
aks_vnet_address_spaces = [
  "10.202.72.0/21",
  "fd0a:7204:c37f:900::/56"
]
subnet_address_prefixes = {
  aks_syspool  = ["fd0a:7204:c37f:901::/64", "10.202.72.0/24"]
  aks_workpool = ["fd0a:7204:c37f:902::/64", "10.202.73.0/24"]
}
kubernetes_version       = "1.33"
team_name                = "auth"
environment              = "at22"
flux_release_tag         = "at_ring2"
developer_entra_id_group = "416302ed-fbab-41a4-8c8d-61f486fa79ca" # Altinn-30-Test-developers
