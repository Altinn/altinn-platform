resource "azurerm_user_assigned_identity" "disapim_identity" {
  name                = var.user_assigned_identity_name != "" ? var.user_assigned_identity_name : "dis-apim-${var.apim_service_name}"
  resource_group_name = var.kubernetes_node_resource_group
  location            = var.kubernetes_node_location
  tags                = var.tags
}

resource "azurerm_federated_identity_credential" "disapim_fic" {
  name                = "dis-apim-aks-${var.apim_service_name}"
  resource_group_name = azurerm_user_assigned_identity.disapim_identity.resource_group_name
  audience            = ["api://AzureADTokenExchange"]
  issuer              = var.kubernetes_cluster_oidc_issuer_url
  subject             = "system:serviceaccount:${var.target_namespace}:dis-apim-operator-controller-manager"
  parent_id           = azurerm_user_assigned_identity.disapim_identity.id
}

resource "azurerm_role_assignment" "disapim_service_operator_role_assignment" {
    scope = var.apim_id
    role_definition_id = "312a565d-c81f-4fd8-895a-4e21e48d571c" # API Management Service Contributor https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles/integration#api-management-service-contributor
    principal_id = azurerm_user_assigned_identity.disapim_identity.principal_id
}
