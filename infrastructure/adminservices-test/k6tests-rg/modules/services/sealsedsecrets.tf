resource "helm_release" "sealed_secrets" {
  lint             = true
  name             = "sealedsecrets"
  namespace        = "sealedsecrets-system"
  create_namespace = true
  repository       = "https://bitnami.github.io/sealed-secrets"
  chart            = "sealed-secrets"
  version          = "2.19.0"
}
