locals {
  namespaces = concat(
    ["platform"], [
      for v
      in var.k8s_rbac :
      v["namespace"]
  ])

  deploy_envs = [
    {
      name : "at22",
      env_type : "dev",
      suffix : "cloud"
    },
    {
      name : "at23",
      env_type : "dev",
      suffix : "cloud"
    },
    {
      name : "at24",
      env_type : "dev",
      suffix : "cloud"
    },
    {
      name : "yt01",
      env_type : "perf",
      suffix : "cloud"
    },
    {
      name : "tt02",
      env_type : "staging",
      suffix : "no"
    },
    {
      name : "prod",
      env_type : "prod",
      suffix : "no"
    },
  ]

  # Hacky-ish, might be easier to deploy to a single namespace and have a controller syncing into other namespaces
  namespaces_deployenvs = distinct(flatten(
    [for n in local.namespaces :
      [for d in local.deploy_envs :
        {
          namespace  = n
          deploy_env = d
        }
      ]
    ]
  ))
}

resource "kubernetes_config_map_v1" "deploy_environments_manifests" {
  depends_on = [kubernetes_namespace.namespace]
  for_each = {
    for entry in local.namespaces_deployenvs :
    "${entry.namespace}.${entry.deploy_env.name}"
    => entry
  }
  metadata {
    name      = "deploy-environments-${each.value.deploy_env.name}"
    namespace = each.value.namespace
  }
  data = {
    BASE_URL = each.value.deploy_env.name == "prod" ? "https://platform.altinn.no" : "https://platform.${each.value.deploy_env.name}.altinn.${each.value.deploy_env.suffix}"
  }
}
