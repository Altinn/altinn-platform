resource "kubernetes_cluster_role_v1" "dev_access" {
  metadata {
    name = "dev-access"
  }

  rule {
    api_groups = [""]
    resources  = ["configmaps"]
    verbs      = ["get", "list", "watch", "delete"]
  }
  rule {
    api_groups = [""]
    resources  = ["pods"]
    verbs      = ["get", "list", "watch"]
  }
  rule {
    api_groups = ["k6.io"]
    resources  = ["testruns"]
    verbs      = ["get", "list", "watch", "create", "update", "patch", "delete"]
  }
  rule {
    api_groups = [""]
    resources  = ["secrets"]
    verbs      = ["list", "watch"]
  }
  rule {
    api_groups = ["bitnami.com"]
    resources  = ["sealedsecrets"]
    verbs      = ["list", "watch", "delete"]
  }
}

resource "kubernetes_cluster_role_v1" "sp_access" {
  metadata {
    name = "github-sp-access"
  }

  rule {
    api_groups = [""]
    resources  = ["configmaps"]
    verbs      = ["create", "update", "delete", "get"]
  }
  rule {
    api_groups = ["bitnami.com"]
    resources  = ["sealedsecrets"]
    verbs      = ["create", "update"]
  }
  rule {
    api_groups = ["k6.io"]
    resources  = ["testruns"]
    verbs      = ["create", "update", "get", "list", "watch", "delete"]
  }
  rule {
    api_groups = [""]
    resources  = ["pods"]
    verbs      = ["get", "list"]
  }
  rule {
    api_groups = [""]
    resources  = ["pods/log"]
    verbs      = ["get", "list"]
  }
}

resource "kubernetes_role_binding_v1" "dev_access" {
  depends_on = [kubernetes_namespace.namespace]
  for_each   = var.k8s_rbac

  metadata {
    name      = "dev-access"
    namespace = each.value.namespace
  }
  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "dev-access"
  }
  subject {
    kind      = "Group"
    namespace = each.value.namespace
    name      = each.value.dev_group
  }
}

resource "kubernetes_role_binding_v1" "sp_access" {
  depends_on = [kubernetes_namespace.namespace]
  for_each   = var.k8s_rbac

  metadata {
    name      = "github-sp-access"
    namespace = each.value.namespace
  }
  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "github-sp-access"
  }
  subject {
    kind      = "Group"
    namespace = each.value.namespace
    name      = each.value.sp_group
  }
}
