resource "azuread_application" "app" {
  display_name     = "${var.prefix}-${var.environment}-otel-collector"
  sign_in_audience = "AzureADMyOrg"
}

resource "azuread_service_principal" "sp" {
  client_id = azuread_application.app.client_id
}

resource "azuread_application_federated_identity_credential" "obs_fed_identity" {
  application_id = azuread_application.app.id
  display_name   = "fed-identity-${var.prefix}-${var.environment}-obs"
  description    = "The federated identity used to federate K8s with Azure AD for ${var.prefix}-${var.environment}-otel"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = var.oidc_issuer_url
  subject        = "system:serviceaccount:monitoring:otel-collector"
}
