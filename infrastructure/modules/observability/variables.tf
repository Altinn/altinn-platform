variable "environment" {
  type        = string
  description = "Environment for resources"
  validation {
    condition     = length(var.environment) > 0
    error_message = "You must provide a value for environment."
  }
}

variable "location" {
  type        = string
  default     = "norwayeast"
  description = "Default region for resources"
}

variable "prefix" {
  type        = string
  description = "Prefix for resource names"
  validation {
    condition     = length(var.prefix) > 0
    error_message = "You must provide a value for prefix for name generation."
  }
}

variable "azurerm_resource_group_obs_name" {
  type        = string
  default     = null
  description = "Name of the existing observability resource group. If provided, the module will use this resource instead of creating a new one."
}

variable "log_analytics_workspace_name" {
  type        = string
  default     = null
  description = "Name of the existing Log Analytics workspace. If provided, the module will use this resource instead of creating a new one."
}

variable "log_analytics_retention_days" {
  type        = number
  default     = 30
  description = "Number of days to retain logs in Log Analytics Workspace."
}

variable "app_insights_name" {
  type        = string
  default     = null
  description = "Name of the existing Application Insights instance. If provided, the module will use this resource instead of creating a new one."
}

variable "app_insights_app_type" {
  type        = string
  default     = "web"
  description = "Application type for Application Insights. Common values: web, other."
}

variable "monitor_workspace_name" {
  type        = string
  default     = null
  description = "Name of the existing Azure Monitor workspace. If provided, the module will use this resource instead of creating a new one."
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "oidc_issuer_url" {
  type        = string
  description = "Oidc issuer url needed for federation"
  validation {
    condition     = length(var.oidc_issuer_url) > 0
    error_message = "You must provide a value for oidc_issuer_url."
  }
}

variable "enable_aks_monitoring" {
  type        = bool
  description = "Should monitoring of a AKS cluster be enabled. If true azurerm_kubernetes_cluster_id is required."
}



variable "azurerm_kubernetes_cluster_id" {
  type        = string
  default     = null
  description = "AKS cluster resource id"
  validation {
    condition     = var.enable_aks_monitoring == false || (var.enable_aks_monitoring == true && var.azurerm_kubernetes_cluster_id != null)
    error_message = "You must provide a value for azurerm_kubernetes_cluster_id when enable_aks_monitoring is true."
  }
}
