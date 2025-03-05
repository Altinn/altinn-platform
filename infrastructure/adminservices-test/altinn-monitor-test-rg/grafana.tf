resource "azurerm_dashboard_grafana" "grafana" {
  name                              = "altinn-grafana-test"
  resource_group_name               = azurerm_resource_group.rg.name
  location                          = azurerm_resource_group.rg.location
  grafana_major_version             = 10
  api_key_enabled                   = true
  deterministic_outbound_ip_enabled = true
  public_network_access_enabled     = true

  identity {
    type = "SystemAssigned"
  }

  azure_monitor_workspace_integrations {
    resource_id = azurerm_monitor_workspace.altinn_monitor.id
  }

  azure_monitor_workspace_integrations {
    resource_id = azurerm_monitor_workspace.k6tests_amw.id
  }
}

resource "azurerm_role_assignment" "tf_grafana_admin" {
  scope                            = azurerm_dashboard_grafana.grafana.id
  role_definition_name             = "Grafana Admin"
  principal_id                     = data.azurerm_client_config.current.object_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "monitoring_reader_rg" {
  scope                            = azurerm_resource_group.rg.id
  role_definition_id               = "/subscriptions/${split("/", azurerm_monitor_workspace.altinn_monitor.id)[2]}/providers/Microsoft.Authorization/roleDefinitions/43d0d8ad-25c7-4714-9337-8ba259a9fe05"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "grafana_identity_reader" {
  scope                            = "/subscriptions/${data.azurerm_client_config.current.subscription_id}"
  role_definition_name             = "Monitoring Reader"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

# Dialogporten
resource "azurerm_role_assignment" "monitoring_reader_rg_dp_test" {

  for_each                         = data.azurerm_resource_group.rd_dp_test
  scope                            = each.value.id
  role_definition_id               = "/subscriptions/${split("/", azurerm_monitor_workspace.altinn_monitor.id)[2]}/providers/Microsoft.Authorization/roleDefinitions/43d0d8ad-25c7-4714-9337-8ba259a9fe05"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "monitoring_reader_rg_dp_stag" {

  scope                            = data.azurerm_resource_group.rg_dp_stag.id
  role_definition_id               = "/subscriptions/${split("/", azurerm_monitor_workspace.altinn_monitor.id)[2]}/providers/Microsoft.Authorization/roleDefinitions/43d0d8ad-25c7-4714-9337-8ba259a9fe05"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "monitoring_reader_rg_dp_prod" {

  scope                            = data.azurerm_resource_group.rg_dp_prod.id
  role_definition_id               = "/subscriptions/${split("/", azurerm_monitor_workspace.altinn_monitor.id)[2]}/providers/Microsoft.Authorization/roleDefinitions/43d0d8ad-25c7-4714-9337-8ba259a9fe05"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "grafana_identity_reader_rg_dp_test" {
  scope                            = "/subscriptions/8a353de8-d81d-468d-a40d-f3574b6bb3f4"
  role_definition_name             = "Monitoring Reader"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "grafana_identity_reader_rg_dp_stag" {
  scope                            = "/subscriptions/e4926efc-0577-47b3-9c3d-757925630eca"
  role_definition_name             = "Monitoring Reader"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "grafana_identity_reader_rg_dp_prod" {
  scope                            = "/subscriptions/c595f787-450d-4c57-84fa-abc5f95d5459"
  role_definition_name             = "Monitoring Reader"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

# Studio
resource "azurerm_role_assignment" "monitoring_reader_rg_studio_law_test" {

  scope                            = data.azurerm_resource_group.rg_studio_law_test.id
  role_definition_id               = "/subscriptions/${split("/", azurerm_monitor_workspace.altinn_monitor.id)[2]}/providers/Microsoft.Authorization/roleDefinitions/43d0d8ad-25c7-4714-9337-8ba259a9fe05"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "monitoring_reader_rg_studio_law_prod" {

  scope                            = data.azurerm_resource_group.rg_studio_law_prod.id
  role_definition_id               = "/subscriptions/${split("/", azurerm_monitor_workspace.altinn_monitor.id)[2]}/providers/Microsoft.Authorization/roleDefinitions/43d0d8ad-25c7-4714-9337-8ba259a9fe05"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "monitoring_reader_rg_studio_dev" {

  scope                            = data.azurerm_resource_group.rg_studio_dev.id
  role_definition_id               = "/subscriptions/${split("/", azurerm_monitor_workspace.altinn_monitor.id)[2]}/providers/Microsoft.Authorization/roleDefinitions/43d0d8ad-25c7-4714-9337-8ba259a9fe05"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "monitoring_reader_rg_studio_stag" {

  scope                            = data.azurerm_resource_group.rg_studio_stag.id
  role_definition_id               = "/subscriptions/${split("/", azurerm_monitor_workspace.altinn_monitor.id)[2]}/providers/Microsoft.Authorization/roleDefinitions/43d0d8ad-25c7-4714-9337-8ba259a9fe05"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "monitoring_reader_rg_studio_prod" {

  scope                            = data.azurerm_resource_group.rg_studio_prod.id
  role_definition_id               = "/subscriptions/${split("/", azurerm_monitor_workspace.altinn_monitor.id)[2]}/providers/Microsoft.Authorization/roleDefinitions/43d0d8ad-25c7-4714-9337-8ba259a9fe05"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "grafana_identity_reader_rg_studio_test" {
  scope                            = "/subscriptions/971ddbb1-27d0-4cc7-a016-461dab5cec05"
  role_definition_name             = "Monitoring Reader"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "grafana_identity_reader_rg_studio_prod" {
  scope                            = "/subscriptions/f66298ed-870c-40e0-bb74-6db89c1a364b"
  role_definition_name             = "Monitoring Reader"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}


locals {
  altinn_30_appmigration_test_developers  = "8ea6868e-317e-45b5-8437-464fb8e48e7e"
  altinn_30_broker_prod_developers        = "7708786a-aa50-4ce8-9f7f-e85459357de1"
  altinn_30_broker_test_developers        = "9b99f951-3873-4310-8baf-464b4da43f26"
  altinn_30_correspondence_prod_developer = "89627577-7e88-446b-a64b-699a9208343c"
  altinn_30_correspondence_test_developer = "12b73376-8726-493c-8d27-aa87e5213e6b"
  altinn_30_developers                    = "416302ed-fbab-41a4-8c8d-61f486fa79ca"
  altinn_30_developers_prod               = "2d962017-75cf-47f2-a76e-50591fbf7fe9"
  altinn_30_operations                    = "143ed28a-6e6d-4ca0-8273-eecb9c1665ba"
  altinn_30_operations_prod               = "5a5ed585-9f7c-4b94-80af-9ceee8124db3"
  dialogporten_developers                 = "857b3aa1-bde3-469c-a052-a24c81503646"
  dialogporten_developers_prod            = "415cfc7b-40f6-4540-9aef-cb9c9050aada"

  grafana_editor = [
    local.altinn_30_appmigration_test_developers,
    local.altinn_30_broker_prod_developers,
    local.altinn_30_broker_test_developers,
    local.altinn_30_correspondence_prod_developer,
    local.altinn_30_correspondence_test_developer,
    local.altinn_30_developers,
    local.altinn_30_developers_prod,
    local.dialogporten_developers,
    local.dialogporten_developers_prod
  ]
  grafana_admin = [local.altinn_30_operations, local.altinn_30_operations_prod]
}

resource "azurerm_role_assignment" "grafana_admin" {
  for_each                         = { for value in local.grafana_admin : value => value if value != null }
  scope                            = azurerm_dashboard_grafana.grafana.id
  role_definition_name             = "Grafana Admin"
  principal_id                     = each.key
  principal_type                   = "Group"
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "grafana_editors" {
  for_each                         = { for value in local.grafana_editor : value => value if value != null }
  scope                            = azurerm_dashboard_grafana.grafana.id
  role_definition_name             = "Grafana Editor"
  principal_id                     = each.value
  principal_type                   = "Group"
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "log_analytics_reader" {
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  scope                            = azurerm_log_analytics_workspace.application.id
  role_definition_name             = "Log Analytics Reader"
  skip_service_principal_aad_check = true
}

# Dialogporten
resource "azurerm_role_assignment" "log_analytics_reader_dp_stag" {
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  scope                            = data.azurerm_log_analytics_workspace.dp_law_stag.id
  role_definition_name             = "Log Analytics Reader"
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "log_analytics_reader_dp_prod" {
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  scope                            = data.azurerm_log_analytics_workspace.dp_law_prod.id
  role_definition_name             = "Log Analytics Reader"
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "log_analytics_reader_dp_test" {
  for_each = data.azurerm_log_analytics_workspace.dp_law_test

  scope                            = each.value.id
  role_definition_name             = "Log Analytics Reader"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}

# Studio
resource "azurerm_role_assignment" "log_analytics_reader_studio_prod" {
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  scope                            = data.azurerm_log_analytics_workspace.studio_law_prod.id
  role_definition_name             = "Log Analytics Reader"
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "log_analytics_reader_studio_test" {
  scope                            = data.azurerm_log_analytics_workspace.studio_law_test.id
  role_definition_name             = "Log Analytics Reader"
  principal_id                     = azurerm_dashboard_grafana.grafana.identity[0].principal_id
  skip_service_principal_aad_check = true
}
