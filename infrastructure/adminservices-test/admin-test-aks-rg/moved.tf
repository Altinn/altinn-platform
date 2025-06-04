moved {
  from = azapi_resource.traefik
  to   = module.aks_resources.azapi_resource.traefik
}
moved {
  from = azurerm_kubernetes_cluster.aks
  to   = module.aks.azurerm_kubernetes_cluster.aks
}
moved {
  from = azurerm_kubernetes_cluster_extension.flux_ext
  to   = module.aks.azurerm_kubernetes_cluster_extension.flux
}
moved {
  from = azurerm_kubernetes_cluster_node_pool.workpool
  to   = module.aks.azurerm_kubernetes_cluster_node_pool.workpool
}
moved {
  from = azurerm_public_ip.pip4
  to   = module.aks.azurerm_public_ip.pip4
}
moved {
  from = azurerm_public_ip.pip6
  to   = module.aks.azurerm_public_ip.pip6
}
moved {
  from = azurerm_public_ip_prefix.prefix4
  to   = module.aks.azurerm_public_ip_prefix.prefix4
}
moved {
  from = azurerm_public_ip_prefix.prefix6
  to   = module.aks.azurerm_public_ip_prefix.prefix6
}
moved {
  from = azurerm_subnet.subnets["aks_syspool"]
  to   = module.aks.azurerm_subnet.aks["aks_syspool"]
}
moved {
  from = azurerm_subnet.subnets["aks_workpool"]
  to   = module.aks.azurerm_subnet.aks["aks_workpool"]
}
moved {
  from = azurerm_virtual_network.vnet
  to   = module.aks.azurerm_virtual_network.aks
}
moved {
  from = azurerm_resource_group.rg
  to   = module.aks.azurerm_resource_group.aks
}
moved {
  from = azurerm_role_assignment.altinncr_acrpull
  to   = module.aks.azurerm_role_assignment.aks_acrpull["/subscriptions/a6e9ee7d-2b65-41e1-adfb-0c8c23515cf9/resourceGroups/acr/providers/Microsoft.ContainerRegistry/registries/altinncr"]
}
