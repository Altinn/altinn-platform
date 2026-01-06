output "log_analytics_workspace_id" {
  value = local.law.id
}

output "monitor_workspace_id" {
  value = local.amw.id
}

output "application_insights_id" {
  value = local.ai.id
}

output "key_vault_uri" {
  value     = azurerm_key_vault.obs_kv.vault_uri
  sensitive = true
}

output "obs_client_id" {
  value     = azuread_application.app.client_id
  sensitive = true
}

output "lakmus_client_id" {
  value     = azuread_application.lakmus_app.client_id
  sensitive = true
}

output "monitor_workspace_write_endpoint" {
  value = var.enable_aks_monitoring ? "" : "${azurerm_monitor_data_collection_endpoint.amw[0].metrics_ingestion_endpoint}/dataCollectionRules/${azurerm_monitor_data_collection_rule.amw[0].immutable_id}/streams/Microsoft-PrometheusMetrics/api/v1/write?api-version=2023-04-24"
  description = "Metrics ingestion endpoint url. If enable_aks_monitor is set to false this will return an empty string"
}
