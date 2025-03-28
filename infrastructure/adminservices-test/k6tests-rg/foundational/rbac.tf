resource "azurerm_role_assignment" "azure_kubernetes_service_cluster_user_role" {
  scope                = azurerm_kubernetes_cluster.k6tests.id
  role_definition_name = "Azure Kubernetes Service Cluster User Role"
  principal_id         = "b95b1fc9-7f21-49c3-8932-07161cd9ac5a"
}
