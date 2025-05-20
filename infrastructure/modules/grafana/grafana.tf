resource "azurerm_resource_group" "grafana" {
  name     = "grafana-${var.organization}-${var.environment}-rg"
  location = var.location
}

resource "azurerm_dashboard_grafana" "grafana" {
  name                              = "grafana-${var.organization}-${var.environment}"
  resource_group_name               = azurerm_resource_group.grafana.name
  location                          = azurerm_resource_group.grafana.location
  api_key_enabled                   = true
  deterministic_outbound_ip_enabled = true
  grafana_major_version             = var.grafana_major_version

  identity {
    type = "SystemAssigned"
  }
}

resource "azurerm_role_assignment" "grafana_admin" {
  scope                            = azurerm_dashboard_grafana.grafana.id
  role_definition_name             = "Grafana Admin"
  principal_id                     = var.grafana_admin_sp_object_id
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
