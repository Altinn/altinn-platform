resource "azuread_application" "dis_apim_app" {
  display_name     = "${var.name_prefix}-dis-apim-app"
  sign_in_audience = "AzureADMyOrg"
}

resource "azuread_service_principal" "dis_apim_sp" {
  client_id = azuread_application.dis_apim_app.client_id
}

resource "azuread_application_federated_identity_credential" "dis_apim_app_fed_identity" {
  application_id = azuread_application.dis_apim_app.id
  display_name   = "dis-apim-fed-identity-${var.name_prefix}-aks"
  description    = "The federated identity used to federate K8s with Azure AD with the app service running in ${var.name_prefix} aks"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = azurerm_kubernetes_cluster.aks.oidc_issuer_url
  subject        = "system:serviceaccount:dis-apim-operator-system:dis-apim-operator-controller-manager"
}
