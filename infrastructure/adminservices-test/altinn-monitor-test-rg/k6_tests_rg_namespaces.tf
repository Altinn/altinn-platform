import {
  to = kubernetes_namespace.dialogporten
  id = "dialogporten"
}

resource "kubernetes_namespace" "dialogporten" {
  metadata {
    name = "dialogporten"
  }
}

import {
  to = kubernetes_namespace.correspondence
  id = "correspondence"
}

resource "kubernetes_namespace" "correspondence" {
  metadata {
    name = "correspondence"
  }
}


import {
  to = kubernetes_namespace.core
  id = "core"
}

resource "kubernetes_namespace" "core" {
  metadata {
    name = "core"
  }
}


import {
  to = kubernetes_namespace.authentication
  id = "authentication"
}

resource "kubernetes_namespace" "authentication" {
  metadata {
    name = "authentication"
  }
}

import {
  to = kubernetes_namespace.platform
  id = "platform"
}

resource "kubernetes_namespace" "platform" {
  metadata {
    name = "platform"
  }
}

locals {
  subset_namespaces = setsubtract(
    [for v in var.k8s_rbac : v["namespace"]],
    ["dialogporten", "correspondence", "core", "authentication", "platform"]
  )
}

resource "kubernetes_namespace" "namespace" {
  for_each = local.subset_namespaces
  metadata {
    name = each.value
  }
}
