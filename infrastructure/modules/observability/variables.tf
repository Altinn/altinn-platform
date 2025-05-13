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
  default     = ""
  description = "Optional explicit name of the observability resource group"
}

variable "log_analytics_workspace_name" {
  type        = string
  default     = ""
  description = "Name for the Log Analytics workspace."
}

variable "log_analytics_retention_days" {
  type    = number
  default = 30
}

variable "app_insights_name" {
  type        = string
  default     = ""
  description = "Name for the Application Insights instance."
}

variable "app_insights_app_type" {
  type    = string
  default = "web"
}

variable "monitor_workspace_name" {
  type        = string
  default     = ""
  description = "Name for the Azure Monitor workspace."
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "kubeconfig_path" {
  type        = string
  default     = "~/.kube/config"
  description = "Path to the kubeconfig that reaches your cluster."
}

variable "kube_context" {
  type        = string
  default     = ""
  description = "Optional kube-context; leave blank to use the current context."
}

variable "oidc_issuer_url" {
  type        = string
  description = "Oidc issuer url needed for federation"
  validation {
    condition     = length(var.oidc_issuer_url) > 0
    error_message = "You must provide a value for oidc_issuer_url."
  }
}
