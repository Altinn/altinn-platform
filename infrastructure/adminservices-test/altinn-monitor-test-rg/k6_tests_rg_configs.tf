locals {
  namespaces = concat(
    ["platform"], [
      for v
      in var.k8s_rbac :
      v["namespace"]
  ])
  deploy_envs = [
    "at21",
    "at22",
    "at23",
    "at24",
    "tt02",
    "yt01",
    "prod",
  ]

  namespaces_deployenvs = distinct(flatten([
    for n in local.namespaces : [
      for d in local.deploy_envs : {
        namespace  = n
        deploy_env = d
      }
    ]
  ]))
}

resource "kubernetes_config_map_v1" "deploy_environments_manifests" {
  for_each = { for entry in local.namespaces_deployenvs : "${entry.namespace}.${entry.deploy_env}" => entry }
  metadata {
    name      = "deploy-environments-${each.value.deploy_env}"
    namespace = each.value.namespace
  }
  data = {
    BASE_URL = each.value.deploy_env == "prod" ? "https://platform.altinn.no/" : "https://platform.${each.value.deploy_env}.altinn.no/"
  }
}
