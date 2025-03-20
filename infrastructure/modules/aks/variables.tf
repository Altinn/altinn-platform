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

variable "aks_sku_tier" {
  type        = string
  default     = "Free"
  description = "Kubernetes SKU"
}

variable "environment" {
  type        = string
  description = "Environment for resources"
  validation {
    condition     = length(var.environment) > 0
    error_message = "You must provide a value for environment."
  }
}

variable "kubernetes_version" {
  type        = string
  description = "Kubernetes version"
  validation {
    condition     = length(var.kubernetes_version) > 0
    error_message = "You must provide kubernetes version in format x.y or x.y.z."
  }
}

variable "location" {
  type        = string
  default     = "norwayeast"
  description = "Default region for resources"
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

variable "prefix" {
  type        = string
  default     = ""
  description = "Prefix for resource names"
  validation {
    condition     = length(var.prefix) > 0
    error_message = "You must provide a value for prefix for name generation."
  }
}

variable "subscription_id" {
  type        = string
  description = "Subscription ID to deploy services"
  validation {
    condition     = length(var.subscription_id) > 0
    error_message = "You must provide a subscription id where the resources will be deployed."
  }
}

variable "subnet_address_prefixes" {
  type        = map(list(string))
  description = "List of subnets"
  validation {
    condition     = length(var.subnet_address_prefixes) > 0
    error_message = "You must provide subnet address prefixes with ipv4 and ipv6 addresses for aks_syspool and aks_workpool."
  }
}

variable "vnet_address_space" {
  type        = list(string)
  description = "VNet address space"
  validation {
    condition     = length(var.vnet_address_space) > 0
    error_message = "You must provide a vnet address space with ipv4 and ipv6 addresses."
  }
}

# Optional explicit variables to override values derived from prefix and environment
variable "azurerm_kubernetes_cluster_aks_dns_service_ip" {
  type        = string
  default     = ""
  description = "Optional explicit aks dns service ip"
}

variable "azurerm_kubernetes_cluster_aks_name" {
  type        = string
  default     = ""
  description = "Optional explicit name of the AKS cluster"
}

variable "azurerm_kubernetes_cluster_aks_pod_cidrs" {
  type        = list(string)
  default     = []
  description = "Optional explicit aks service cidrs"
}

variable "azurerm_kubernetes_cluster_aks_service_cidrs" {
  type        = list(string)
  default     = []
  description = "Optional explicit aks service cidrs"
}

variable "azurerm_log_analytics_workspace_aks_name" {
  type        = string
  default     = ""
  description = "Optional explicit name of the log analytics workspace"
}

variable "azurerm_monitor_workspace_aks_name" {
  type        = string
  default     = ""
  description = "Optional explicit name of the monitor workspace"
}

variable "azurerm_public_ip_prefix_prefix4_name" {
  type        = string
  default     = ""
  description = "Optional explicit name of the public ipv4 prefix"
}

variable "azurerm_public_ip_prefix_prefix6_name" {
  type        = string
  default     = ""
  description = "Optional explicit name of the public ipv6 prefix"
}

variable "azurerm_resource_group_aks_name" {
  type        = string
  default     = ""
  description = "Optional explicit name of the AKS resource group"
}

variable "azurerm_resource_group_monitor_name" {
  type        = string
  default     = ""
  description = "Optional explicit name of the monitor resource group"
}

variable "azurerm_storage_account_aks_name" {
  type        = string
  default     = ""
  description = "Optional explicit name of the AKS Log storage account"
}

variable "azurerm_virtual_network_aks_name" {
  type        = string
  default     = ""
  description = "Optional explicit name of the AKS virtual network"
}

variable "azurerm_virtual_public_ip_pip4_name" {
  type        = string
  default     = ""
  description = "Optional explicit name of the public ipv4"
}

variable "azurerm_virtual_public_ip_pip6_name" {
  type        = string
  default     = ""
  description = "Optional explicit name of the public ipv6"
}
