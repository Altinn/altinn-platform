resource "helm_release" "prometheus_push_gateway" {
  depends_on = [
    helm_release.kube_prometheus_stack,
  ]
  lint             = true
  name             = "prometheus-pushgateway"
  namespace        = "monitoring"
  create_namespace = false
  repository       = "https://prometheus-community.github.io/helm-charts"
  chart            = "prometheus-pushgateway"
  skip_crds        = true
  version          = "3.6.1"

  values = [
    "${templatefile(
      "${path.module}/k6_tests_rg_kube_prometheus_push_gateway_values.tftpl",
      {
      }
    )}"
  ]
}
