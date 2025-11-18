resource "helm_release" "prometheus_operator_crds" {
  depends_on = [
    azurerm_kubernetes_cluster.k6tests
  ]
  lint       = true
  name       = "prometheus-operator-crds"
  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "prometheus-operator-crds"
  version    = "23.0.0"
}

data "azurerm_monitor_data_collection_rule" "prometheus" {
  name                = "k6tests-amw"
  resource_group_name = "MA_k6tests-amw_norwayeast_managed"
}

resource "helm_release" "kube_prometheus_stack" {
  depends_on = [
    helm_release.prometheus_operator_crds,
    azuread_application.prometheus,
    azurerm_monitor_workspace.k6tests_amw
  ]
  lint             = true
  name             = "kube-prometheus-stack"
  namespace        = "monitoring"
  create_namespace = true
  repository       = "https://prometheus-community.github.io/helm-charts"
  chart            = "kube-prometheus-stack"
  skip_crds        = true
  version          = "79.5.0"

  values = [
    "${templatefile(
      "${path.module}/k6_tests_rg_kube_prometheus_stack_values.tftpl",
      {
        cluster_name          = "${azurerm_kubernetes_cluster.k6tests.name}",
        client_id             = "${azuread_application.prometheus.client_id}",
        tenant_id             = "${data.azurerm_client_config.current.tenant_id}",
        remote_write_endpoint = "https://k6tests-amw-0vej.norwayeast-1.metrics.ingest.monitor.azure.com/dataCollectionRules/dcr-81e9cf1b38fb4648b047399c5593ebda/streams/Microsoft-PrometheusMetrics/api/v1/write?api-version=2023-04-24"
      }
    )}"
  ]
}

resource "azuread_application" "prometheus" {
  display_name     = "adminservicestest-k6tests-prometheus"
  sign_in_audience = "AzureADMyOrg"
}

resource "azuread_service_principal" "prometheus" {
  client_id = azuread_application.prometheus.client_id
}

resource "azuread_application_federated_identity_credential" "prometheus" {
  application_id = azuread_application.prometheus.id
  display_name   = "adminservicestest-k6tests-prometheus"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = azurerm_kubernetes_cluster.k6tests.oidc_issuer_url
  subject        = "system:serviceaccount:monitoring:kube-prometheus-stack-prometheus"
}

resource "azurerm_role_assignment" "monitoring_metrics_publisher" {
  scope                = data.azurerm_monitor_data_collection_rule.prometheus.id
  role_definition_name = "Monitoring Metrics Publisher"
  principal_id         = azuread_service_principal.prometheus.object_id
}
