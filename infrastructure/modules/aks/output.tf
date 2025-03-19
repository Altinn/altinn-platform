output "aks_identity" {
  value       = azurerm_kubernetes_cluster.aks.identity
  description = "Managed Service Identity that is configured on this Kubernetes Cluster"
}

output "aks_kubelet_identity" {
  value       = azurerm_kubernetes_cluster.aks.kubelet_identity
  description = "Managed Identity assigned to the Kubelets"
}

output "aks_name" {
  value       = azurerm_kubernetes_cluster.aks.name
  description = "The name of the managed Kubernetes Cluster"
}

output "aks_node_resource_group" {
  value       = azurerm_kubernetes_cluster.aks.node_resource_group
  description = "The name of the Resource Group in which the managed Kubernetes Cluster exists"
}

output "aks_oidc_issuer_url" {
  value       = azurerm_kubernetes_cluster.aks.oidc_issuer_url
  description = "The OIDC issuer URL that is associated with the cluster"
}

output "kube_admin_config" {
  value       = azurerm_kubernetes_cluster.aks.kube_admin_config
  description = "Base64 encoded cert/key/user/pass used by clients to authenticate to the Kubernetes cluster"
}

output "pip4_ip_address" {
  value       = azurerm_public_ip.pip4.ip_address
  description = "The IPv4 address value that was allocated"
}

output "pip6_ip_address" {
  value       = azurerm_public_ip.pip6.ip_address
  description = "The IPv6 address value that was allocated"
}
