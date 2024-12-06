resource "helm_release" "k6_operator" {
  name             = "k6-operator"
  namespace        = "k6-operator-system"
  create_namespace = true
  repository       = "https://grafana.github.io/helm-charts"
  chart            = "k6-operator"
  version          = "3.10.1"
  values           = ["${file("${path.module}/k6_tests_rg_k6_operator_values.yaml")}"]
}
