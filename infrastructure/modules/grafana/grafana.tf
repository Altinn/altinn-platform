# Create resource group only if create_resource_group is true
resource "azurerm_resource_group" "grafana" {
  count    = var.create_resource_group ? 1 : 0
  name     = var.resource_group_name != "" ? var.resource_group_name : "grafana-${var.prefix}-${var.environment}-rg"
  location = var.location
  tags = merge(var.localtags, {
    submodule = "grafana"
  })
}

resource "azurerm_dashboard_grafana" "grafana" {
  name                              = var.dashboard_name != "" ? var.dashboard_name : "grafana-${var.prefix}-${var.environment}"
  resource_group_name               = var.create_resource_group ? azurerm_resource_group.grafana[0].name : var.resource_group_name
  location                          = var.location
  api_key_enabled                   = true
  deterministic_outbound_ip_enabled = true
  grafana_major_version             = var.grafana_major_version
  tags = merge(var.localtags, {
    submodule = "grafana"
  })

  dynamic "azure_monitor_workspace_integrations" {
    for_each = var.monitor_workspace_ids
    content {
      resource_id = azure_monitor_workspace_integrations.value
    }
  }

  identity {
    type = "SystemAssigned"
  }
}

resource "azurerm_role_assignment" "grafana_admin_sp" {
  scope                            = azurerm_dashboard_grafana.grafana.id
  role_definition_name             = "Grafana Admin"
  principal_id                     = var.client_config_current_object_id
  skip_service_principal_aad_check = true
}

resource "grafana_service_account" "admin" {
  depends_on = [azurerm_role_assignment.grafana_admin]
  name       = "admin-service-account"
  role       = "Admin"
}

resource "grafana_service_account_token" "grafana_operator" {
  name               = "grafana-operator"
  service_account_id = grafana_service_account.admin.id
}
