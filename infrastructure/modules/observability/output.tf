output "log_analytics_workspace_id" {
  value = azurerm_log_analytics_workspace.obs.id
}

output "monitor_workspace_id" {
  value = azurerm_monitor_workspace.obs.id
}

output "application_insights_id" {
  value = azurerm_application_insights.obs.id
}
