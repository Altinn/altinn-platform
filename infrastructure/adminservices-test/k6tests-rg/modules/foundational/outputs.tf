output "k6tests_cluster_name" {
  value = azurerm_kubernetes_cluster.k6tests.name
}

output "k6tests_resource_group_name" {
  value = azurerm_resource_group.k6tests.name
}

output "k6tests_resource_group_location" {
  value = azurerm_resource_group.k6tests.location
}
