variable "prefix" {
  type        = string
  description = "Prefix for resource names"
}
variable "environment" {
  type        = string
  description = "Environment for resources"
}
variable "subscription_id" {
  type        = string
  description = "Subscription id to deploy services"
}
variable "location" {
  type        = string
  description = "Default region for resources"
}
variable "aks_sku_tier" {
  type        = string
  description = "Kubernetes sku"
}
variable "kubernetes_version" {
  type        = string
  description = "Kubernetes version"
}
variable "vnet_address_space" {
  type        = list(string)
  description = "vnet adress space"
}
variable "subnet_address_prefixes" {
  type        = map(list(string))
  description = "list of subnets"
}
variable "pool_configs" {
  type = map(object({
    vm_size              = string
    auto_scaling_enabled = string
    node_count           = string
    min_count            = string
    max_count            = string
  }))
  description = "variables for nodepools"
}
variable "aks_acrpull_scopes" {
  type        = list(string)
  description = "List of aks acrpull scopes"
  validation {
    condition     = length(var.aks_acrpull_scopes) > 0
    error_message = "You must provide at least one ACR resource ID."
  }
}
