variable "aks_node_resource_group" {
  type        = string
  description = "AKS node resource group name"
}

variable "azurerm_kubernetes_cluster_id" {
  type        = string
  description = "AKS cluster resource id"
}

variable "flux_release_tag" {
  type        = string
  default     = "latest"
  description = "OCI image that Flux should watch and reconcile"
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

variable "obs_kv_uri" {
  type        = string
  description = "Key vault uri for observability"
}

variable "obs_client_id" {
  type        = string
  description = "Client id for the obs app"
}

variable "obs_tenant_id" {
  type        = string
  description = "Tenant id for the obs app"
}

variable "environment" {
  type        = string
  description = "Environment"
  validation {
    condition     = length(var.environment) > 0
    error_message = "You must provide a value for environment."
  }
}

variable "syncroot_namespace" {
  type        = string
  default     = ""
  description = "The namespace to use for the syncroot. This is the containing 'folder' in altinncr repo and the namespace in the cluster."
  validation {
    condition     = length(var.syncroot_namespace) > 0
    error_message = "You must provide a value for syncroot_namespace."
  }
}
