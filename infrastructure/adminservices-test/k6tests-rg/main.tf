resource "random_string" "suffix" {
  length  = 4
  special = false
}

module "foundational" {
  source     = "./modules/foundational"
  tenant_id  = local.tenant_id
  namespaces = local.namespaces

  k8s_admin_group_object_ids = local.k8s_admin_group_object_ids
  k8s_users_group_object_id  = local.k8s_users_group_object_id

  log_analytics_workspace_daily_quota_gb    = local.log_analytics_workspace_daily_quota_gb
  log_analytics_workspace_retention_in_days = local.log_analytics_workspace_retention_in_days

  suffix = "-${random_string.suffix.result}"
}

module "services" {
  depends_on = [module.foundational]
  source     = "./modules/services"

  tenant_id = local.tenant_id

  k8s_rbac = local.k8s_rbac

  k6tests_cluster_name = local.k6tests_cluster_name
  oidc_issuer_url      = local.oidc_issuer_url

  remote_write_endpoint = local.remote_write_endpoint

  suffix = "-${random_string.suffix.result}"
}
