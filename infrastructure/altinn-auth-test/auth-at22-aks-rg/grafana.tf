module "grafana" {
  source                          = "../../modules/grafana"
  prefix                          = local.team_name
  environment                     = local.environment
  tenant_id                       = local.tenant_id
  client_config_current_object_id = data.azurerm_client_config.current.object_id
  monitor_workspace_id = [
    module.observability.monitor_workspace_id
  ]
  grafana_admin_access  = []
  grafana_editor_access = []
}
