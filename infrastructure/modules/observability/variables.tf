variable "app_insights_app_type" {
  type        = string
  default     = "web"
  description = "Application type for Application Insights. Common values: web, other."
}

variable "reuse_application_insights" {
  type        = bool
  default     = false
  description = "Set true to reuse an existing Application Insights instance (pass app_insights_connection_string)."
}

variable "app_insights_connection_string" {
  type        = string
  default     = null
  sensitive   = true
  description = "Connection string of an existing Application Insights when reusing."
  validation {
    condition     = var.reuse_application_insights ? (var.app_insights_connection_string != null && trimspace(var.app_insights_connection_string) != "") : (var.app_insights_connection_string == null)
    error_message = "app_insights_connection_string must be non-empty when reuse_application_insights is true, and null when false."
  }
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

variable "azurerm_resource_group_obs_name" {
  type        = string
  default     = null
  description = "Name of the existing observability resource group. If provided, the module will use this resource instead of creating a new one."
}

variable "ci_service_principal_object_id" {
  type        = string
  description = "Object ID of the CI service principal used for role assignments."
  validation {
    condition     = length(trimspace(var.ci_service_principal_object_id)) > 0
    error_message = "You must provide a value for ci_service_principal_object_id."
  }
}

variable "enable_aks_monitoring" {
  type        = bool
  description = "Should monitoring of a AKS cluster be enabled. If true azurerm_kubernetes_cluster_id is required."
}

variable "environment" {
  type        = string
  description = "Environment for resources"
  validation {
    condition     = length(trimspace(var.environment)) > 0
    error_message = "You must provide a value for environment."
  }
}

variable "localtags" {
  type        = map(string)
  description = "A map of tags to assign to the created resources."
  default     = {}
}

variable "location" {
  type        = string
  default     = "norwayeast"
  description = "Default region for resources"
}

variable "log_analytics_retention_days" {
  type        = number
  default     = 30
  description = "Number of days to retain logs in Log Analytics Workspace."
}

variable "reuse_log_analytics_workspace" {
  type        = bool
  default     = false
  description = "Set true to reuse an existing Log Analytics Workspace (pass log_analytics_workspace_id)."
}

variable "log_analytics_workspace_id" {
  type        = string
  default     = null
  description = "ID of an existing Log Analytics Workspace when reusing."
  validation {
    condition     = var.reuse_log_analytics_workspace ? (var.log_analytics_workspace_id != null && trimspace(var.log_analytics_workspace_id) != "") : (var.log_analytics_workspace_id == null)
    error_message = "log_analytics_workspace_id must be non-empty when reuse_log_analytics_workspace is true, and null when false."
  }
}

variable "reuse_monitor_workspace" {
  type        = bool
  default     = false
  description = "Set true to reuse an existing Azure Monitor Workspace (pass monitor_workspace_name and monitor_workspace_id)."
}

variable "monitor_workspace_id" {
  type        = string
  default     = null
  description = "ID of an existing Azure Monitor Workspace when reusing."
  validation {
    condition     = var.reuse_monitor_workspace ? (var.monitor_workspace_id != null && trimspace(var.monitor_workspace_id) != "") : (var.monitor_workspace_id == null)
    error_message = "monitor_workspace_id must be non-empty when reuse_monitor_workspace is true, and null when false."
  }
}

variable "monitor_workspace_name" {
  type        = string
  default     = null
  description = "Name of an existing Azure Monitor Workspace when reusing."
  validation {
    condition     = var.reuse_monitor_workspace ? (var.monitor_workspace_name != null && trimspace(var.monitor_workspace_name) != "") : (var.monitor_workspace_name == null)
    error_message = "monitor_workspace_name must be non-empty when reuse_monitor_workspace is true, and null when false."
  }
}

variable "oidc_issuer_url" {
  type        = string
  description = "Oidc issuer url needed for federation"
  validation {
    condition     = length(trimspace(var.oidc_issuer_url)) > 0
    error_message = "You must provide a value for oidc_issuer_url."
  }
}

variable "prefix" {
  type        = string
  description = "Prefix for resource names"
  validation {
    condition     = length(trimspace(var.prefix)) > 0
    error_message = "You must provide a value for prefix for name generation."
  }
}

variable "subscription_id" {
  type        = string
  description = "Azure subscription ID for resource deployments."
  validation {
    condition     = length(trimspace(var.subscription_id)) > 0
    error_message = "You must provide a value for subscription_id."
  }
}

variable "tenant_id" {
  type        = string
  description = "Azure AD tenant ID for resource configuration."
  validation {
    condition     = length(trimspace(var.tenant_id)) > 0
    error_message = "You must provide a value for tenant_id."
  }
}
