resource "helm_release" "sealed_secrets" {
  depends_on       = [azurerm_kubernetes_cluster.k6tests]
  lint             = true
  name             = "sealedsecrets"
  namespace        = "sealedsecrets-system"
  create_namespace = true
  repository       = "https://bitnami-labs.github.io/sealed-secrets"
  chart            = "sealed-secrets"
  version          = "2.17.4"
}
