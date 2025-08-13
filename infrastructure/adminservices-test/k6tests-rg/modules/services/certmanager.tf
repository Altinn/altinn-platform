resource "helm_release" "certmanager" {
  // depends_on = [  ]
  lint             = true
  name             = "certmanager"
  namespace        = "certmanager"
  create_namespace = true
  repository       = "https://charts.jetstack.io"
  chart            = "cert-manager" // jetstack/cert-manager
  version          = "1.18.2"

  set = [
    {
      name  = "crds.enabled"
      value = "true"
    }
  ]
}
