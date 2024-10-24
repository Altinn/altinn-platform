
resource "azurerm_log_analytics_workspace" "application" {
  name                = "altinn-monitor-test-law"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  retention_in_days   = 30
  identity {
    type = "SystemAssigned"
  }
}

resource "azurerm_role_assignment" "operations_altinn_law_reader" {
  depends_on                       = [azurerm_log_analytics_workspace.application]
  for_each                         = { for value in local.grafana_admin : value => value if value != null }
  scope                            = azurerm_log_analytics_workspace.application.id
  role_definition_name             = "Log Analytics Reader"
  principal_id                     = each.key
  principal_type                   = "Group"
  skip_service_principal_aad_check = true
}