# Give Grafana access read monitoring in subscriptions
resource "azurerm_role_assignment" "grafana_permission" {
  for_each = {
    for value in var.grafana_monitor_reader_subscription_id : value => value if value != null && trim(value) != ""
  }
  scope                            = each.value
  role_definition_name             = "Monitoring Reader"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

# Give Grafana read access to Azure Monitor workspaces
resource "azurerm_role_assignment" "amw_datareaderrole" {
  for_each = {
    for value in var.monitor_workspace_id : value => value if value != null && trim(value) != ""
  }
  scope              = each.value
  role_definition_id = "/subscriptions/${split("/", each.value)[2]}/providers/Microsoft.Authorization/roleDefinitions/b0d8363b-8ddd-447d-831f-62ca05bff136"
  principal_id       = azurerm_dashboard_grafana.grafana.identity[0].principal_id
}

# Give users access to Grafana
resource "azurerm_role_assignment" "grafana_admin" {
  for_each = {
    for value in var.grafana_admin_access : value => value if value != null && trim(value) != ""
  }
  scope                            = azurerm_dashboard_grafana.grafana.id
  role_definition_name             = "Grafana Admin"
  principal_id                     = each.key
  principal_type                   = "Group"
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "grafana_editor" {
  for_each = {
    for value in var.grafana_editor_access : value => value if value != null && trim(value) != ""
  }
  scope                            = azurerm_dashboard_grafana.grafana.id
  role_definition_name             = "Grafana Editor"
  principal_id                     = each.value
  principal_type                   = "Group"
  skip_service_principal_aad_check = true
}
