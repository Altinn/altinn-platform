resource "helm_release" "traefik" {
  depends_on = [
    azurerm_kubernetes_cluster.k6tests
  ]
  lint             = true
  name             = "traefik"
  namespace        = "traefik"
  create_namespace = true
  repository       = "https://traefik.github.io/charts"
  chart            = "traefik"
  version          = "39.0.8"
  values = [
    "${templatefile("${path.module}/k6_tests_rg_traefik_values.tftpl", {})}"
  ]
}
