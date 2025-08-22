resource "azuread_application" "cert_manager_app" {
  display_name     = "${var.prefix}-${var.environment}-cert-manager"
  sign_in_audience = "AzureADMyOrg"
}

resource "azuread_service_principal" "cert_manager_sp" {
  client_id = azuread_application.cert_manager_app.client_id
}

resource "azuread_application_federated_identity_credential" "cert_manager_fed_identity" {
  application_id = azuread_application.cert_manager_app.id
  display_name   = "fed-identity-${var.prefix}-${var.environment}-cert-manager"
  description    = "The federated identity used to federate K8s with Azure AD for ${var.prefix}-${var.environment}-cert-manager"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = var.oidc_issuer_url
  subject        = "system:serviceaccount:cert-manager:cert-manager"
}

# Gives key vault reader to the whole subscription
resource "azurerm_role_assignment" "dns_zone_contributor_cert_manager" {
  scope                            = azurerm_dns_zone.child_zone.id
  role_definition_name             = "DNS Zone Contributor"
  principal_id                     = azuread_service_principal.cert_manager_sp.object_id
  skip_service_principal_aad_check = true
}
