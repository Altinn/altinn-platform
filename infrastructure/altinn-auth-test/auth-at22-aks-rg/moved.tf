moved {
  from = module.infra-resources.azapi_resource.grafana_manifests
  to   = module.infra-resources.azapi_resource.grafana_manifests[0]
}
moved {
  from = module.infra-resources.azapi_resource.grafana_operator
  to   = module.infra-resources.azapi_resource.grafana_operator[0]
}
