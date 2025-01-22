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
    verbs      = ["create", "update", "delete"]
  }
  rule {
    api_groups = ["bitnami.com"]
    resources  = ["sealedsecrets"]
    verbs      = ["create", "update"]
  }
  rule {
    api_groups = ["k6.io"]
    resources  = ["testruns"]
    verbs      = ["create", "update", "get", "watch", "delete"]
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

resource "kubernetes_role_binding_v1" "dialogporten_dev_access" {
  metadata {
    name      = "dev-access"
    namespace = "dialogporten"
  }
  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "dev-access"
  }
  subject {
    kind      = "Group"
    namespace = "dialogporten"
    name      = "c403060d-5c8a-41b0-8c19-84fa60d0ce18"
  }
}

resource "kubernetes_role_binding_v1" "dialogporten_sp_access" {
  metadata {
    name      = "github-sp-access"
    namespace = "dialogporten"
  }
  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "ClusterRole"
    name      = "github-sp-access"
  }
  subject {
    kind      = "Group"
    namespace = "dialogporten"
    name      = "b22b612d-9dc5-4f8b-8816-e551749bd19c"
  }
}
