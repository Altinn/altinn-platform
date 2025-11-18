locals {
  namespaces = concat(["platform"], [for v in var.k8s_rbac : v["namespace"]])
}

resource "kubernetes_config_map_v1" "deploy_environment_at22" {
  depends_on = [kubernetes_namespace.namespace]
  for_each   = toset(local.namespaces)
  metadata {
    name      = "deploy-environments-at22"
    namespace = each.key
  }
  data = {
    BASE_URL            = "https://platform.at22.altinn.cloud"
    ALTINN2_BASE_URL    = "https://at22.altinn.cloud"
    ALTINN_CDN_BASE_URL = "https://altinncdn.no"
    AM_UI_BASE_URL      = "https://am.ui.at22.altinn.cloud"
    DEPLOY_ENV          = "at22"
    ENV_TYPE            = "dev"
  }
}

resource "kubernetes_config_map_v1" "deploy_environment_at23" {
  depends_on = [kubernetes_namespace.namespace]
  for_each   = toset(local.namespaces)
  metadata {
    name      = "deploy-environments-at23"
    namespace = each.key
  }
  data = {
    BASE_URL            = "https://platform.at23.altinn.cloud"
    ALTINN2_BASE_URL    = "https://at23.altinn.cloud"
    ALTINN_CDN_BASE_URL = "https://altinncdn.no"
    AM_UI_BASE_URL      = "https://am.ui.at23.altinn.cloud"
    DEPLOY_ENV          = "at23"
    ENV_TYPE            = "dev"
  }
}

resource "kubernetes_config_map_v1" "deploy_environment_at24" {
  depends_on = [kubernetes_namespace.namespace]
  for_each   = toset(local.namespaces)
  metadata {
    name      = "deploy-environments-at24"
    namespace = each.key
  }
  data = {
    BASE_URL            = "https://platform.at24.altinn.cloud"
    ALTINN2_BASE_URL    = "https://at24.altinn.cloud"
    ALTINN_CDN_BASE_URL = "https://altinncdn.no"
    AM_UI_BASE_URL      = "https://am.ui.at24.altinn.cloud"
    DEPLOY_ENV          = "at24"
    ENV_TYPE            = "dev"
  }
}

resource "kubernetes_config_map_v1" "deploy_environment_yt01" {
  depends_on = [kubernetes_namespace.namespace]
  for_each   = toset(local.namespaces)
  metadata {
    name      = "deploy-environments-yt01"
    namespace = each.key
  }
  data = {
    BASE_URL            = "https://platform.yt01.altinn.cloud"
    ALTINN2_BASE_URL    = "https://yt01.ai.basefarm.net"
    ALTINN_CDN_BASE_URL = "https://altinncdn.no"
    AM_UI_BASE_URL      = "https://am.ui.yt01.altinn.cloud"
    DEPLOY_ENV          = "yt01"
    ENV_TYPE            = "perf"
  }
}

resource "kubernetes_config_map_v1" "deploy_environment_tt02" {
  depends_on = [kubernetes_namespace.namespace]
  for_each   = toset(local.namespaces)
  metadata {
    name      = "deploy-environments-tt02"
    namespace = each.key
  }
  data = {
    BASE_URL            = "https://platform.tt02.altinn.no"
    ALTINN2_BASE_URL    = "https://tt02.altinn.no"
    ALTINN_CDN_BASE_URL = "https://altinncdn.no"
    AM_UI_BASE_URL      = "https://am.ui.tt02.altinn.no"
    DEPLOY_ENV          = "tt02"
    ENV_TYPE            = "staging"
  }
}

resource "kubernetes_config_map_v1" "deploy_environment_prod" {
  depends_on = [kubernetes_namespace.namespace]
  for_each   = toset(local.namespaces)
  metadata {
    name      = "deploy-environments-prod"
    namespace = each.key
  }
  data = {
    BASE_URL            = "https://platform.altinn.no"
    ALTINN2_BASE_URL    = "https://altinn.no"
    ALTINN_CDN_BASE_URL = "https://altinncdn.no"
    AM_UI_BASE_URL      = "https://am.ui.altinn.no"
    DEPLOY_ENV          = "prod"
    ENV_TYPE            = "prod"
  }
}
