resource "azurerm_user_assigned_identity" "dispgsql_identity" {
  name                = var.user_assigned_identity_name != "" ? var.user_assigned_identity_name : "dis-pgsql-${var.name}-${var.environment}"
  resource_group_name = var.resource_group_name
  location            = var.location
  tags                = var.tags
}

resource "azurerm_federated_identity_credential" "dispgsql_fic" {
  name                = "dis-pgsql-aks-${var.name}-${var.environment}"
  resource_group_name = azurerm_user_assigned_identity.dispgsql_identity.resource_group_name
  parent_id           = azurerm_user_assigned_identity.dispgsql_identity.id

  audience = ["api://AzureADTokenExchange"]
  issuer   = var.oidc_issuer_url
  subject  = "system:serviceaccount:dis-pgsql-operator-system:dis-pgsql-operator-controller-manager"
}

resource "azurerm_role_assignment" "dispgsql_network_reader" {
  scope                = azurerm_virtual_network.postgresql.id
  role_definition_name = "Reader"
  principal_id         = azurerm_user_assigned_identity.dispgsql_identity.principal_id
}
