resource "helm_release" "pyrra" {
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
