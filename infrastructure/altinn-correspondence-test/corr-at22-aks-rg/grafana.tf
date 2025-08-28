module "grafana" {
  depends_on                      = [module.observability]
  source                          = "../../modules/grafana"
  prefix                          = var.team_name
  environment                     = var.environment
  client_config_current_object_id = data.azurerm_client_config.current.object_id
  monitor_workspace_ids = {
    "default-obs-workspace" : module.observability.monitor_workspace_id
  }
  grafana_admin_access = [
    "143ed28a-6e6d-4ca0-8273-eecb9c1665ba", # Altinn-30-Test-Operations
  ]
  grafana_editor_access = [
    "416302ed-fbab-41a4-8c8d-61f486fa79ca", # Altinn-30-Test-developers
    "12b73376-8726-493c-8d27-aa87e5213e6b", # Altinn-30-Correspondence-Test-Developers
    "9b99f951-3873-4310-8baf-464b4da43f26", # Altinn-30-Broker-Test-Developers
  ]
  grafana_monitor_reader_subscription_id = [
    var.subscription_id,
  ]
}
