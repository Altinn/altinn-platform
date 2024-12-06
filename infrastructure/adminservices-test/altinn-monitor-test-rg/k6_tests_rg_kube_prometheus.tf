resource "helm_release" "prometheus_operator_crds" {
  name       = "prometheus-operator-crds"
  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "prometheus-operator-crds"
  version    = "16.0.1"
}

resource "helm_release" "kube_prometheus_stack" {
  depends_on       = [helm_release.prometheus_operator_crds]
  name             = "kube-prometheus-stack"
  namespace        = "monitoring"
  create_namespace = true
  repository       = "https://prometheus-community.github.io/helm-charts"
  chart            = "kube-prometheus-stack"
  skip_crds        = true
  version          = "66.3.1"

  values = [
    "${templatefile("${path.module}/k6_tests_rg_kube_prometheus_stack_values.tftpl", {
    cluster_name = "${azurerm_kubernetes_cluster.k6tests.name}" })}"
  ]
}
