resource "azurerm_application_insights" "app" {
  name                = "altinn-monitor-test-insights"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  application_type    = "web"
  disable_ip_masking  = true
  retention_in_days   = 30
  workspace_id        = azurerm_log_analytics_workspace.application.id

}

resource "azurerm_role_assignment" "operations_altinn_monitoring_contributor" {
  depends_on                       = [azurerm_application_insights.app]
  for_each                         = { for value in local.grafana_admin : value => value if value != null }
  scope                            = azurerm_application_insights.app.id
  role_definition_name             = "Monitoring Contributor"
  principal_id                     = each.key
  principal_type                   = "Group"
  skip_service_principal_aad_check = true
}
