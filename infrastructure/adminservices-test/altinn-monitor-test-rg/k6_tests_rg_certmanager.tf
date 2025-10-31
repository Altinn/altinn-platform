resource "helm_release" "certmanager" {
  depends_on = [
    azurerm_kubernetes_cluster.k6tests
  ]
  lint             = true
  name             = "certmanager"
  namespace        = "certmanager"
  create_namespace = true
  repository       = "https://charts.jetstack.io"
  chart            = "cert-manager" // jetstack/cert-manager
  version          = "v1.19.1"

  values = [
    "${templatefile("${path.module}/k6_tests_rg_certmanager_values.tftpl", {})}"
  ]
}
