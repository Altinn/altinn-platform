resource "helm_release" "grafana_k8s_monitoring" {
  depends_on = [
    helm_release.loki,
  ]
  lint             = true
  name             = "k8s-monitoring"
  namespace        = "monitoring"
  take_ownership   = true
  create_namespace = false
  repository       = "https://grafana.github.io/helm-charts"
  chart            = "k8s-monitoring"
  version          = "3.7.3"

  values = [
    "${templatefile(
      "${path.module}/k6_tests_rg_grafana_k8s_monitoring_values.tftpl",
      {
        cluster_name = "${azurerm_kubernetes_cluster.k6tests.name}",
        namespaces   = toset([for v in var.k8s_rbac : v["namespace"]])
      }
    )}"
  ]
}
