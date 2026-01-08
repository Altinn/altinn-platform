resource "helm_release" "k6_operator" {
  depends_on       = [helm_release.prometheus_operator_crds]
  lint             = true
  name             = "k6-operator"
  namespace        = "k6-operator-system"
  create_namespace = true
  repository       = "https://grafana.github.io/helm-charts"
  chart            = "k6-operator"
  version          = "4.2.0"
  values           = [file("${path.module}/k6_operator_values.yaml")]
}
