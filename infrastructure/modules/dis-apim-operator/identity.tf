resource "azurerm_user_assigned_identity" "disapim_identity" {
  name                = var.azurerm_user_assigned_identity_name != "" ? var.azurerm_user_assigned_identity_name : "dis-apim-${var.dis_apim_service_name}"
  resource_group_name = var.azurerm_kubernetes_node_resource_group
  location            = var.azurerm_kubernetes_node_location
  tags                = var.azurerm_tags
}

resource "azurerm_federated_identity_credential" "disapim_fic" {
  name                = "dis-apim-aks-${var.dis_apim_service_name}"
  resource_group_name = azurerm_user_assigned_identity.disapim_identity.resource_group_name
  audience            = ["api://AzureADTokenExchange"]
  issuer              = var.azurerm_kubernetes_cluster_oidc_issuer_url
  subject             = "system:serviceaccount:${var.dis_apim_target_namespace}:dis-apim-operator-controller-manager"
  parent_id           = azurerm_user_assigned_identity.disapim_identity.id
}

resource "azurerm_role_assignment" "disapim_service_operator_role_assignment" {
    scope = var.azurerm_apim_id
    role_definition_id = "312a565d-c81f-4fd8-895a-4e21e48d571c" # API Management Service Contributor https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles/integration#api-management-service-contributor
    principal_id = azurerm_user_assigned_identity.disapim_identity.principal_id
}
