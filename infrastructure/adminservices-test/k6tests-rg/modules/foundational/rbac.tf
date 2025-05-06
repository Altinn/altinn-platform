resource "azurerm_role_assignment" "azure_kubernetes_service_cluster_user_role" {
  scope                = azurerm_kubernetes_cluster.k6tests.id
  role_definition_name = "Azure Kubernetes Service Cluster User Role"
  principal_id         = var.k8s_users_group_object_id
}

resource "azurerm_role_assignment" "reader_user_role" {
  scope                = azurerm_log_analytics_workspace.k6tests.id
  role_definition_name = "Reader"
  principal_id         = var.k8s_users_group_object_id
}
