resource "helm_release" "grafana_k8s_monitoring" {
  depends_on = [
    helm_release.loki,
  ]
  name             = "k8s-monitoring"
  namespace        = "monitoring"
  create_namespace = false
  repository       = "https://grafana.github.io/helm-charts"
  chart            = "k8s-monitoring"
  version          = "3.1.5"

  values = [
    "${templatefile(
      "${path.module}/grafana_k8s_monitoring_values.tftpl",
      {
        cluster_name = "${azurerm_kubernetes_cluster.k6tests.name}",
      }
    )}"
  ]
}
