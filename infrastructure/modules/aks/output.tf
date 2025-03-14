output "aks_node_resource_group" {
  value = azurerm_kubernetes_cluster.aks.node_resource_group
}
output "pip4_ip_address" {
  value = azurerm_public_ip.pip4.ip_address
}
output "pip6_ip_address" {
  value = azurerm_public_ip.pip6.ip_address
}
output "aks_kube_config_client_certificate" {
  value = azurerm_kubernetes_cluster.aks.kube_admin_config.0.client_certificate
}
output "aks_kube_config_client_key" {
  value = azurerm_kubernetes_cluster.aks.kube_admin_config.0.client_key
}
output "aks_kube_config_cluster_ca_certificate" {
  value = azurerm_kubernetes_cluster.aks.kube_admin_config.0.cluster_ca_certificate
}
output "aks_kube_config_host" {
  value = azurerm_kubernetes_cluster.aks.kube_admin_config.0.host
}
