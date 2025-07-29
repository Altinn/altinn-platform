resource "helm_release" "pyrra" {
  depends_on       = [azurerm_kubernetes_cluster.k6tests]
  lint             = true
  name             = "pyrra"
  namespace        = "pyrra-system"
  create_namespace = true
  repository       = "https://rlex.github.io/helm-charts"
  chart            = "pyrra"
  version          = "0.14.3"
  set = [
    {
      name  = "genericRules.enabled"
      value = "true"
    }
  ]
}
