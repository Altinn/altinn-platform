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

# # Assign "Storage Blob Data Contributor" Role to Terraform running user
# resource "azurerm_role_assignment" "terraform_blob_contributor" {
#   scope                            = azurerm_resource_group.monitor.id
#   role_definition_name             = "Storage Blob Data Contributor"
#   principal_id                     = data.azurerm_client_config.current.object_id
#   skip_service_principal_aad_check = true
# }
