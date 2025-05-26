variable "aks_node_resource_group" {
  type        = string
  description = "AKS node resource group name"
}

variable "azurerm_kubernetes_cluster_id" {
  type        = string
  description = "AKS cluster resource id"
}

variable "environment" {
  type        = string
  description = "Environment"
  validation {
    condition     = length(var.environment) > 0
    error_message = "You must provide a value for environment."
  }
}

variable "flux_release_tag" {
  type        = string
  default     = "latest"
  description = "OCI image that Flux should watch and reconcile"
}

variable "grafana_endpoint" {
  type        = string
  description = "URL endpoint for Grafana dashboard access"
  validation {
    condition     = length(var.grafana_endpoint) > 0
    error_message = "You must provide a value for grafana_endpoint."
  }
}

variable "obs_client_id" {
  type        = string
  description = "Client id for the obs app"
}

variable "obs_kv_uri" {
  type        = string
  description = "Key vault uri for observability"
}

variable "obs_tenant_id" {
  type        = string
  description = "Tenant id for the obs app"
}

variable "pip4_ip_address" {
  type        = string
  description = "AKS ipv4 public ip"
}

variable "pip6_ip_address" {
  type        = string
  description = "AKS ipv6 public ip"
}

variable "subnet_address_prefixes" {
  type = object({
    aks_syspool  = list(string)
    aks_workpool = list(string)
  })
  description = "list of subnets"
}

variable "syncroot_namespace" {
  type        = string
  description = "The namespace to use for the syncroot. This is the containing 'folder' in altinncr repo and the namespace in the cluster."
  validation {
    condition     = length(var.syncroot_namespace) > 0
    error_message = "You must provide a value for syncroot_namespace."
  }
}

variable "token_grafana_operator" {
  type        = string
  sensitive   = true
  description = "Authentication token for Grafana operator to manage Grafana resources"
  validation {
    condition     = length(var.token_grafana_operator) > 0
    error_message = "You must provide a value for token_grafana_operator."
  }
}
