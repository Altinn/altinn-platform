resource "azurerm_user_assigned_identity" "disapim_identity" {
  name                = var.azurerm_user_assigned_identity_name != "" ? var.azurerm_user_assigned_identity_name : "dis-apim-${var.dis_apim_service_name}"
  resource_group_name = var.azurerm_kubernetes_node_resource_group
  location            = var.azurerm_kubernetes_node_location
  tags                = var.azurerm_tags
}

resource "azurerm_federated_identity_credential" "aso_fic" {
  name                = "dis-apim-aks-${var.dis_apim_service_name}"
  resource_group_name = azurerm_user_assigned_identity.disapim_identity.resource_group_name
  audience            = ["api://AzureADTokenExchange"]
  issuer              = var.azurerm_kubernetes_cluster_oidc_issuer_url
  subject             = "system:serviceaccount:${var.dis_apim_target_namespace}:dis-apim-operator-controller-manager"
  parent_id           = azurerm_user_assigned_identity.disapim_identity.id
}