data "azuread_client_config" "current" {
  count = var.smtp == null ? 0 : 1
}

resource "azuread_application" "smtp" {
  count            = var.smtp == null ? 0 : 1
  display_name     = "${local.grafana_name}-smtp"
  sign_in_audience = "AzureADMyOrg"
}

resource "azuread_service_principal" "smtp" {
  count     = var.smtp == null ? 0 : 1
  client_id = azuread_application.smtp[0].client_id
}

resource "time_rotating" "smtp_secret" {
  count         = var.smtp == null ? 0 : 1
  rotation_days = var.smtp.secret_rotation_days
}

resource "azuread_application_password" "smtp" {
  count          = var.smtp == null ? 0 : 1
  application_id = azuread_application.smtp[0].id
  display_name   = "terraform"

  rotate_when_changed = {
    rotation = time_rotating.smtp_secret[0].id
  }
}

resource "azapi_resource" "smtp_username" {
  count     = var.smtp == null ? 0 : 1
  type      = "Microsoft.Communication/communicationServices/SmtpUsernames@2025-09-01"
  name      = "smtp-${local.grafana_name}"
  parent_id = var.smtp.communication_service_id

  body = {
    properties = {
      username           = local.grafana_name
      entraApplicationId = azuread_application.smtp[0].client_id
      tenantId           = data.azuread_client_config.current[0].tenant_id
    }
  }
}

resource "azurerm_role_assignment" "smtp_sender" {
  count                            = var.smtp == null ? 0 : 1
  scope                            = var.smtp.communication_service_id
  role_definition_id               = var.smtp.sender_role_definition_id
  principal_id                     = azuread_service_principal.smtp[0].object_id
  principal_type                   = "ServicePrincipal"
  skip_service_principal_aad_check = true
}
