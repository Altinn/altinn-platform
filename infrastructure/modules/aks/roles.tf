# Assign "Network Contributor" Role to AKS Managed Identity
resource "azurerm_role_assignment" "network_contributor" {
  scope                            = azurerm_resource_group.aks.id
  role_definition_name             = "Network Contributor"
  principal_id                     = azurerm_kubernetes_cluster.aks.identity[0].principal_id
  skip_service_principal_aad_check = true
}

# Assign pull permission in listed ACR
resource "azurerm_role_assignment" "aks_acrpull" {
  for_each                         = toset(var.aks_acrpull_scopes)
  principal_id                     = azurerm_kubernetes_cluster.aks.kubelet_identity[0].object_id
  role_definition_name             = "AcrPull"
  scope                            = each.value
  skip_service_principal_aad_check = true

  depends_on = [azurerm_kubernetes_cluster.aks]
}

# Assign Azure Kubernetes Service Cluster User Role to user groups
resource "azurerm_role_assignment" "aks_user_role" {
  for_each = {
    for value in var.aks_user_role_scopes : value => value if value != null
  }
  principal_id                     = each.value
  role_definition_name             = "Azure Kubernetes Service Cluster User Role"
  scope                            = azurerm_kubernetes_cluster.aks.id
  principal_type                   = "Group"
  skip_service_principal_aad_check = true

  depends_on = [azurerm_kubernetes_cluster.aks]
}
