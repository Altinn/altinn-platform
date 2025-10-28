output "log_analytics_workspace_id" {
  value = try(azurerm_log_analytics_workspace.obs[0].id, data.azurerm_log_analytics_workspace.existing[0].id)
}

output "monitor_workspace_id" {
  value = try(azurerm_monitor_workspace.obs[0].id, data.azurerm_monitor_workspace.existing[0].id)
}

output "application_insights_id" {
  value = try(azurerm_application_insights.obs[0].id, data.azurerm_application_insights.existing[0].id)
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
