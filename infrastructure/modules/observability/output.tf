output "log_analytics_workspace_id" {
  value = azurerm_log_analytics_workspace.obs.id
}

output "monitor_workspace_id" {
  value = azurerm_monitor_workspace.obs.id
}

output "application_insights_id" {
  value = azurerm_application_insights.obs.id
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
