resource "helm_release" "ingress_nginx" {
  // depends_on       = []
  lint             = true
  name             = "ingress-nginx"
  namespace        = "ingress-nginx"
  create_namespace = true
  repository       = "https://kubernetes.github.io/ingress-nginx"
  chart            = "ingress-nginx"
  version          = "4.14.1"
  values = [
    "${templatefile("${path.module}/ingress-nginx_values.tftpl", {})}"
  ]
}
