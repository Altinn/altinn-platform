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

resource "azurerm_role_definition" "user_assigned_identity_role" {
  name        = "DIS User Assigned Identity Admin Role"
  scope       = var.dis_resource_group_id
  description = "Role for Dis deployed Azure Service Operator to manage resources in the specified resource group."

  permissions {
    actions = [
      "Microsoft.ManagedIdentity/userAssignedIdentities/read",
			"Microsoft.ManagedIdentity/userAssignedIdentities/write",
			"Microsoft.ManagedIdentity/userAssignedIdentities/delete",
			"Microsoft.ManagedIdentity/userAssignedIdentities/federatedIdentityCredentials/read",
			"Microsoft.ManagedIdentity/userAssignedIdentities/federatedIdentityCredentials/write",
			"Microsoft.ManagedIdentity/userAssignedIdentities/federatedIdentityCredentials/delete",
			"Microsoft.ManagedIdentity/userAssignedIdentities/revokeTokens/action",
			"Microsoft.Authorization/*/read",
    ]
    not_actions = []
  }

  assignable_scopes = [
    var.dis_resource_group_id
  ]
  
}

resource "azurerm_role_assignment" "aso_contrib_role_assignment" {
  scope                = var.dis_resource_group_id
  role_definition_name = azurerm_role_definition.user_assigned_identity_role.name
  principal_id         = azurerm_user_assigned_identity.aso_identity.id
}
