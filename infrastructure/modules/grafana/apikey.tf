resource "azuread_application" "grafana" {
  display_name = "grafana-${var.prefix}-${var.environment}-admin-sp"
  owners       = [data.azuread_client_config.current.object_id]
}

resource "azuread_service_principal" "grafana" {
  client_id                    = azuread_application.grafana.client_id
  app_role_assignment_required = false
}

resource "time_rotating" "grafana_admin_sp_pwd" {
  rotation_days = 365
}

resource "azuread_service_principal_password" "grafana" {
  service_principal_id = azuread_service_principal.grafana.id
  rotate_when_changed = {
    rotation = time_rotating.example.id
  }
}

data "http" "api_key" {
  url = "https://login.microsoftonline.com/${var.tenant_id}/oauth2/token"

  method = "POST"
  request_headers = {
    Content-Type = "application/x-www-form-urlencoded"
  }
  request_body = "grant_type=client_credentials&client_id=${azuread_application.grafana.client_id}&client_secret=${azuread_service_principal_password.grafana.value}&resource=ce34e7e5-485f-4d76-964f-b3d2b16d1e4f"
}

# locals {
#   grafana_bearer_token = jsondecode(data.http.api_key.response_body).access_token
# }

output "grafana_bearer_token" {
  value = jsondecode(data.http.api_key.response_body).access_token
}
