output "grafana_endpoint" {
  value = azurerm_dashboard_grafana.grafana.endpoint
}

output "grafana_bearer_token" {
  value     = jsondecode(data.http.api_key.response_body).access_token
  sensitive = true
}

output "token_grafana_operator" {
  value     = grafana_service_account_token.grafana_operator.key
  sensitive = true
}
