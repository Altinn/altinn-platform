module "grafana" {
  source                          = "../../modules/grafana"
  prefix                          = local.team_name
  environment                     = local.environment
  tenant_id                       = local.tenant_id
  client_config_current_object_id = data.azurerm_client_config.current.object_id
  workspace_integrations = [
    module.aks.azurerm_monitor_workspace_id,
    module.observability.monitor_workspace_id
  ]
}
