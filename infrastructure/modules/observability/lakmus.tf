resource "azuread_application" "lakmus_app" {
  display_name     = "${var.prefix}-${var.environment}-lakmus"
  sign_in_audience = "AzureADMyOrg"
}

resource "azuread_application_password" "lakmus_app_pass" {
  application_id = azuread_application.lakmus_app.id

  rotate_when_changed = {
    rotation = time_rotating.password.id
  }
}

resource "azuread_service_principal" "lakmus_sp" {
  client_id = azuread_application.lakmus_app.client_id
}

resource "azuread_service_principal_password" "lakmus_sp_pass" {
  service_principal_id = azuread_service_principal.lakmus_sp.id
  rotate_when_changed = {
    rotation = time_rotating.password.id
  }
}

resource "azuread_application_federated_identity_credential" "lakmus_fed_identity" {
  application_id = azuread_application.lakmus_app.id
  display_name   = "fed-identity-${var.prefix}-${var.environment}-lakmus"
  description    = "The federated identity used to federate K8s with Azure AD for ${var.prefix}-${var.environment}-lakmus"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = var.oidc_issuer_url
  subject        = "system:serviceaccount:monitoring:lakmus"
}

# Gives key vault reader to the whole subscription
resource "azurerm_role_assignment" "kv_reader_lakmus" {
  scope                            = data.azurerm_client_config.current.subscription_id
  role_definition_name             = "Key Vault Reader"
  principal_id                     = azuread_service_principal.lakmus_sp.object_id
  skip_service_principal_aad_check = true
}
