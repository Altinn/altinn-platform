resource "azurerm_role_assignment" "azure_kubernetes_service_cluster_user_role" {
  scope                = azurerm_kubernetes_cluster.k6tests.id
  role_definition_name = "Azure Kubernetes Service Cluster User Role"
  principal_id         = "b95b1fc9-7f21-49c3-8932-07161cd9ac5a"
}

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

variable "k8s_rbac" {
  type = map(
    object(
      {
        namespace = string
        dev_group = string
        sp_group  = string
      }
    )
  )
  default = {
    dialogporten = {
      namespace = "dialogporten",
      dev_group = "c403060d-5c8a-41b0-8c19-84fa60d0ce18"
      sp_group  = "b22b612d-9dc5-4f8b-8816-e551749bd19c"
    }
    correspondence = {
      namespace = "correspondence"
      dev_group = "954a4d24-8c7e-4382-9861-2b5d1a515253"
      sp_group  = "e36ca3b3-f495-45a5-bca4-4fc83424633f"
    }
    core = {
      namespace = "core"
      dev_group = "4dde4651-a9ca-4df1-9e05-216272284c7d"
      sp_group  = "e87d6f10-6fc0-4a09-a9b0-e8c994ed4052"
    }
    authentication = {
      namespace = "authentication"
      dev_group = "5c42ac79-86e2-46d0-85d3-ae751dd5f057"
      sp_group  = "328cbe61-aeb1-4782-bb36-d288c69b4f15"
    }
  }
}

resource "kubernetes_role_binding_v1" "dev_access" {
  for_each = var.k8s_rbac

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
  for_each = var.k8s_rbac

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
