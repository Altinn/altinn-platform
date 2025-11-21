# TODO: Don't really like this hardcoded name.
resource "azuread_application" "prometheus" {
  display_name     = "adminservicestest-k6tests-prometheus${var.suffix}"
  sign_in_audience = "AzureADMyOrg"
}

resource "azuread_service_principal" "prometheus" {
  client_id = azuread_application.prometheus.client_id
}

# TODO: Don't really like this hardcoded name.
resource "azuread_application_federated_identity_credential" "prometheus" {
  application_id = azuread_application.prometheus.id
  display_name   = "adminservicestest-k6tests-prometheus${var.suffix}"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = var.oidc_issuer_url
  subject        = "system:serviceaccount:monitoring:kube-prometheus-stack-prometheus"
}

resource "helm_release" "prometheus_operator_crds" {
  lint       = true
  name       = "prometheus-operator-crds"
  namespace  = "monitoring"
  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "prometheus-operator-crds"
  version    = "24.0.2"
}

resource "helm_release" "kube_prometheus_stack" {
  depends_on = [
    azuread_application.prometheus,
    helm_release.prometheus_operator_crds,
  ]
  lint             = true
  name             = "kube-prometheus-stack"
  namespace        = "monitoring"
  create_namespace = true
  repository       = "https://prometheus-community.github.io/helm-charts"
  chart            = "kube-prometheus-stack"
  skip_crds        = true
  version          = "79.6.1"

  values = [
    "${templatefile(
      "${path.module}/kube_prometheus_stack_values.tftpl",
      {
        cluster_name          = "${var.k6tests_cluster_name}",
        client_id             = "${azuread_application.prometheus.client_id}",
        tenant_id             = "${var.tenant_id}",
        remote_write_endpoint = "${var.remote_write_endpoint}"
      }
    )}"
  ]
}

resource "azurerm_role_assignment" "monitoring_metrics_publisher" {
  scope                = var.data_collection_rule_id
  role_definition_name = "Monitoring Metrics Publisher"
  principal_id         = azuread_service_principal.prometheus.object_id
}
