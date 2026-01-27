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
      "${path.module}/grafana_k8s_monitoring_values.tftpl",
      {
        cluster_name = "${var.k6tests_cluster_name}",
        namespaces   = toset([for v in var.k8s_rbac : v["namespace"]])
      }
    )}"
  ]
}
