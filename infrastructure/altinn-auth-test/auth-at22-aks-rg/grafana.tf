module "grafana" {
  source                          = "../../modules/grafana"
  prefix                          = local.team_name
  environment                     = local.environment
  client_config_current_object_id = data.azurerm_client_config.current.object_id
  monitor_workspace_id = [
    module.observability.monitor_workspace_id
  ]
  grafana_admin_access = [
    "143ed28a-6e6d-4ca0-8273-eecb9c1665ba", # Altinn-30-Test-Operations
  ]
  grafana_editor_access = [
    "416302ed-fbab-41a4-8c8d-61f486fa79ca", # Altinn-30-Test-developers
  ]
  grafana_monitor_reader_subscription_id = [
    var.subscription_id,
  ]
}
