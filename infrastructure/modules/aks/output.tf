output "aks_identity" {
  value = azurerm_kubernetes_cluster.aks.identity
}

output "aks_kubelet_identity" {
  value = azurerm_kubernetes_cluster.aks.kubelet_identity
}

output "aks_name" {
  value = azurerm_kubernetes_cluster.aks.name
}

output "aks_node_resource_group" {
  value = azurerm_kubernetes_cluster.aks.node_resource_group
}

output "aks_oidc_issuer_url" {
  value = azurerm_kubernetes_cluster.aks.oidc_issuer_url
}

output "kube_admin_config" {
  value = azurerm_kubernetes_cluster.aks.kube_admin_config
}

output "pip4_ip_address" {
  value = azurerm_public_ip.pip4.ip_address
}

output "pip6_ip_address" {
  value = azurerm_public_ip.pip6.ip_address
}
