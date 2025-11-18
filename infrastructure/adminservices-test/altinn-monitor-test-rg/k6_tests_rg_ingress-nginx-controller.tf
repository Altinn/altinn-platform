resource "helm_release" "ingress_nginx" {
  depends_on = [
    azurerm_kubernetes_cluster.k6tests
  ]
  lint             = true
  name             = "ingress-nginx"
  namespace        = "ingress-nginx"
  create_namespace = true
  repository       = "https://kubernetes.github.io/ingress-nginx"
  chart            = "ingress-nginx"
  version          = "4.14.0"
  values = [
    "${templatefile("${path.module}/k6_tests_rg_ingress-nginx_values.tftpl", {})}"
  ]
}
