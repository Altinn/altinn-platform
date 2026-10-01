locals {
  acs_relay_id              = "/subscriptions/a6e9ee7d-2b65-41e1-adfb-0c8c23515cf9/resourceGroups/dis-email-relay-acs-rg/providers/Microsoft.Communication/communicationServices/dis-acs-relay"
  acs_smtp_sender_role_guid = "4f3553a5-c6b2-4074-9a71-0a6f2384b9e0"
  grafana_smtp_username     = "altinn-grafana-test"
}

resource "azuread_application" "grafana_smtp" {
  display_name     = "altinn-grafana-test-smtp"
  sign_in_audience = "AzureADMyOrg"
  owners           = [data.azurerm_client_config.current.object_id]
}

resource "azuread_service_principal" "grafana_smtp" {
  client_id = azuread_application.grafana_smtp.client_id
}

resource "time_rotating" "grafana_smtp_secret" {
  rotation_days = 180
}

resource "azuread_application_password" "grafana_smtp" {
  application_id = azuread_application.grafana_smtp.id
  display_name   = "terraform"

  rotate_when_changed = {
    rotation = time_rotating.grafana_smtp_secret.id
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "azapi_resource" "grafana_smtp_username" {
  type      = "Microsoft.Communication/communicationServices/SmtpUsernames@2025-09-01"
  name      = "smtp-${local.grafana_smtp_username}"
  parent_id = local.acs_relay_id

  body = {
    properties = {
      username           = local.grafana_smtp_username
      entraApplicationId = azuread_application.grafana_smtp.client_id
      tenantId           = data.azurerm_client_config.current.tenant_id
    }
  }
}

resource "azurerm_role_assignment" "grafana_smtp_sender" {
  scope              = local.acs_relay_id
  role_definition_id = "/subscriptions/a6e9ee7d-2b65-41e1-adfb-0c8c23515cf9/providers/Microsoft.Authorization/roleDefinitions/${local.acs_smtp_sender_role_guid}"
  principal_id       = azuread_service_principal.grafana_smtp.object_id
  principal_type     = "ServicePrincipal"
}
