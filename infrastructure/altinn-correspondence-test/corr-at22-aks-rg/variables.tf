variable "subscription_id" {
  type        = string
  description = "Subscription id to deploy services"
}

variable "parent_zone_subscription_id" {
  type        = string
  description = "Subscription id for parent dns zone"
  sensitive   = true
}

variable "aks_vnet_address_spaces" {
  type        = list(string)
  description = "vnet address space reserved for AKS"
}

variable "subnet_address_prefixes" {
  type = object({
    aks_syspool  = list(string)
    aks_workpool = list(string)
  })
  description = "list of subnets"
}

variable "app_access_token" {
  type        = string
  sensitive   = true
  description = "Azure App access token"
  validation {
    condition     = length(var.app_access_token) > 0
    error_message = "You must provide a value for app_access_token from pipeline run."
  }
}

variable "team_name" {
  description = "Name of the team that uses this cluster"
  type        = string
}

variable "environment" {
  description = "Name of the environment to deploy"
  type        = string
}

variable "flux_release_tag" {
  description = "Flux release tag that the infra flux resources will follow"
  type        = string
}
