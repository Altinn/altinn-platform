resource "kubernetes_manifest" "deploy_environments_manifests" {
  for_each = fileset("deploy-environments-manifests/", "*.yaml")
  manifest = yamldecode(file("deploy-environments-manifests/${each.value}"))
}
