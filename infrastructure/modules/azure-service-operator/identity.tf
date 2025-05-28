resource "azurerm_resource_group" "aso_rg" {
  name     = var.azurerm_resource_group_aso_name != "" ? var.azurerm_resource_group_aso_name : "aso-${var.prefix}-${var.environment}-rg"
  location = var.location
  tags     = var.tags
}

resource "azurerm_user_assigned_identity" "aso_identity" {
  name                = var.azurerm_user_assigned_identity_name != "" ? var.azurerm_user_assigned_identity_name : "aso-identity-${var.prefix}-${var.environment}"
  resource_group_name = azurerm_resource_group.aso_rg.name
  location            = azurerm_resource_group.aso_rg.location
  tags                = var.tags
}

resource "azurerm_federated_identity_credential" "aso_fic" {
  name                = "aso-aks-${var.prefix}-${var.environment}"
  resource_group_name = azurerm_resource_group.aso_rg.name
  audience            = ["api://AzureADTokenExchange"]
  issuer              = var.azurerm_kubernetes_cluster_oidc_issuer_url
  subject             = "system:serviceaccount:${var.aso_namespace}:${var.aso_service_account_name}"
  parent_id           = azurerm_user_assigned_identity.aso_identity.id
}
  