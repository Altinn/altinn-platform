variable "admin_group_object_ids" {
  type        = list(string)
  description = "List of group object IDs to get admin access to the cluster"
  validation {
    condition     = length(var.admin_group_object_ids) > 0
    error_message = "You must provide at least one admin group object ID."
  }
}

variable "aks_acrpull_scopes" {
  type        = list(string)
  default     = []
  description = "List of AKS ACR pull scopes"
}

variable "flux_release_tag" {
  type = string
}

variable "grafana_endpoint" {
  type        = string
  description = "URL endpoint for Grafana dashboard access"
  default     = ""
}

variable "kubernetes_version" {
  type = string
}

variable "name_prefix" {
  type = string
}

variable "pool_configs" {
  type = map(object({
    vm_size              = string
    auto_scaling_enabled = bool
    node_count           = number
    min_count            = number
    max_count            = number
  }))
  description = "Variables for node pools"
  validation {
    condition     = length(var.pool_configs) > 0
    error_message = "You must provide pool config for syspool and workpool."
  }
}

variable "subnet_address_prefixes" {
  type = map(list(string))
}

variable "subscription_id" {
  type = string
}

variable "token_grafana_operator" {
  type        = string
  sensitive   = true
  description = "Authentication token for Grafana operator to manage Grafana resources"
  default     = ""
}

variable "vnet_address_space" {
  type = list(string)
}

variable "arm_enable_preflight" {
  type    = bool
  default = true
}
