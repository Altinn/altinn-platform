resource "helm_release" "certmanager" {
  // depends_on = [  ]
  lint             = true
  name             = "certmanager"
  namespace        = "certmanager"
  create_namespace = true
  repository       = "https://charts.jetstack.io"
  chart            = "cert-manager" // jetstack/cert-manager
  version          = "v1.20.0"

  values = [
    "${templatefile("${path.module}/certmanager_values.tftpl", {})}"
  ]
}
