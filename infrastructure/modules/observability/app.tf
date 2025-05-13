resource "time_rotating" "password" {
  rotation_days = 365
}

resource "azuread_application" "app" {
  display_name     = "${var.prefix}-${var.environment}-obs"
  sign_in_audience = "AzureADMyOrg"
}

resource "azuread_application_password" "app" {
  application_id = azuread_application.app.id

  rotate_when_changed = {
    rotation = time_rotating.password.id
  }
}

resource "azuread_service_principal" "sp" {
  client_id = azuread_application.app.client_id
}

resource "azuread_service_principal_password" "sp" {
  service_principal_id = azuread_service_principal.sp.id
  rotate_when_changed = {
    rotation = time_rotating.password.id
  }
}

resource "azuread_application_federated_identity_credential" "obs_fed_identity" {
  application_id = azuread_application.app.id
  display_name   = "fed-identity-${var.prefix}-${var.environment}-obs"
  description    = "The federated identity used to federate K8s with Azure AD for ${var.prefix}-${var.environment}-obs"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = var.oidc_issuer_url
  subject        = "system:serviceaccount:monitoring:otel-collector"
}
