
resource "azurerm_log_analytics_workspace" "application" {
  name                = "altinn-monitor-test-law"
  resource_group_name               = azurerm_resource_group.rg.name
  location                          = azurerm_resource_group.rg.location
  retention_in_days   = 30
  identity {
    type = "SystemAssigned"
  }
}

locals {
  altinn_30_operations      = "143ed28a-6e6d-4ca0-8273-eecb9c1665ba"
  altinn_30_operations_prod = "5a5ed585-9f7c-4b94-80af-9ceee8124db3"
  grafana_admin             = [local.altinn_30_operations, local.altinn_30_operations_prod]
}

resource "azurerm_role_assignment" "operations_altinn_monitoring_contributor" {
    depends_on           = [azurerm_log_analytics_workspace.application]
  for_each                         = { for value in local.grafana_admin : value => value if value != null }
  scope                = azurerm_log_analytics_workspace.application.id
  role_definition_name = "Log Analytics Reader"
  principal_id                     = each.key
  principal_type                   = "Group"
  skip_service_principal_aad_check = true
}