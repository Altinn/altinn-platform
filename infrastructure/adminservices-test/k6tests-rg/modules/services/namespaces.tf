resource "kubernetes_namespace" "namespace" {
  for_each = toset(
    concat(
      ["platform"],
      [for v in var.k8s_rbac : v["namespace"]]
    )
  )
  metadata {
    name = each.value
  }
}
