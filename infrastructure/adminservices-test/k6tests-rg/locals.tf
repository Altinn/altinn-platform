locals {
  tenant_id = data.azurerm_client_config.current.tenant_id

  k8s_admin_group_object_ids = ["c9c317cc-aec0-4c8b-bdad-b54333686e8a"]
  k8s_users_group_object_id  = "b95b1fc9-7f21-49c3-8932-07161cd9ac5a"

  log_analytics_workspace_daily_quota_gb    = 5
  log_analytics_workspace_retention_in_days = 30

  k8s_rbac = {

    authentication = {
      namespace = "authentication"
      dev_group = "5c42ac79-86e2-46d0-85d3-ae751dd5f057"
      sp_group  = "328cbe61-aeb1-4782-bb36-d288c69b4f15"
    }

    core = {
      namespace = "core"
      dev_group = "4dde4651-a9ca-4df1-9e05-216272284c7d"
      sp_group  = "e87d6f10-6fc0-4a09-a9b0-e8c994ed4052"
    }

    correspondence = {
      namespace = "correspondence"
      dev_group = "954a4d24-8c7e-4382-9861-2b5d1a515253"
      sp_group  = "e36ca3b3-f495-45a5-bca4-4fc83424633f"
    }

    dialogporten = {
      namespace = "dialogporten",
      dev_group = "c403060d-5c8a-41b0-8c19-84fa60d0ce18"
      sp_group  = "b22b612d-9dc5-4f8b-8816-e551749bd19c"
    }

    portaler = {
      namespace = "portaler",
      dev_group = "01505bd1-7216-419d-ae24-bdad763d7e06"
      sp_group  = "3b2529e7-8fa6-48d8-a4ce-eb4683d79c0c"
    }
  }
  namespaces                      = toset([for v in local.k8s_rbac : v["namespace"]])
  k6tests_cluster_name            = module.foundational.k6tests_cluster_name
  k6tests_resource_group_name     = module.foundational.k6tests_resource_group_name
  k6tests_resource_group_location = module.foundational.k6tests_resource_group_location
  oidc_issuer_url                 = data.azurerm_kubernetes_cluster.k6tests.oidc_issuer_url

  dce_metrics_ingestion_endpoint = data.azurerm_monitor_data_collection_endpoint.k6tests.metrics_ingestion_endpoint

  dcr_immutable_id      = data.azurerm_monitor_data_collection_rule.k6tests.immutable_id
  remote_write_endpoint = "${local.dce_metrics_ingestion_endpoint}/dataCollectionRules/${local.dcr_immutable_id}/streams/Microsoft-PrometheusMetrics/api/v1/write?api-version=2023-04-24"

  suffix = "-${random_string.suffix.result}"

  data_collection_rule_id = data.azurerm_monitor_data_collection_endpoint.k6tests.id
}
