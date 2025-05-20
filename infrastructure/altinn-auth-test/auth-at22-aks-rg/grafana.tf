module "grafana" {
  source                          = "../../modules/grafana"
  prefix                          = "auth"
  environment                     = "at22"
  tenant_id                       = local.tenant_id
  client_config_current_object_id = data.azurerm_client_config.current.object_id
}
