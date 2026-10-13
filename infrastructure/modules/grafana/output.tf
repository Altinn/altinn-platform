output "grafana_endpoint" {
  value = azurerm_dashboard_grafana.grafana.endpoint
}

output "token_grafana_operator" {
  value     = grafana_service_account_token.grafana_operator.key
  sensitive = true
}

output "smtp_application_client_id" {
  value = try(azuread_application.smtp[0].client_id, null)
}
