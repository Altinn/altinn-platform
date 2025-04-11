variable "suffix" {
  type = string
}

variable "tenant_id" {
  type = string
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
}

variable "k6tests_cluster_name" {
  type = string
}

variable "oidc_issuer_url" {
  type = string
}

variable "remote_write_endpoint" {
  type = string
}

variable "data_collection_rule_id" {
  type = string
}
