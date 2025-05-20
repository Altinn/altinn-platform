module "grafana" {
  source      = "../../modules/grafana"
  prefix      = "auth"
  environment = "at22"
  tenant_id   = local.tenant_id
}
