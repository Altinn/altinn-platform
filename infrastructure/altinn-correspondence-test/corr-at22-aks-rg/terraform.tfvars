subscription_id = "37bac63a-b964-46b2-8de8-ba93c432ea1f"
aks_vnet_address_spaces = [
  "10.203.0.0/20",
  "fd96:20bf:3235::/56"
]
subnet_address_prefixes = {
  aks_syspool  = ["fd96:20bf:3235:1::/64", "10.203.1.0/24"]
  aks_workpool = ["fd96:20bf:3235:2::/64", "10.203.2.0/24"]
}
team_name        = "corr"
environment      = "at22"
flux_release_tag = "at_ring2"