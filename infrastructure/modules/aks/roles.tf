resource "azurerm_role_assignment" "aks_acrpull" {
  for_each                         = toset(var.aks_acrpull_scopes)
  principal_id                     = azurerm_kubernetes_cluster.aks.kubelet_identity[0].object_id
  role_definition_name             = "AcrPull"
  scope                            = each.value
  skip_service_principal_aad_check = true

  depends_on = [azurerm_kubernetes_cluster.aks]
}
