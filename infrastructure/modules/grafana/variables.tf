variable "client_config_current_object_id" {
  type        = string
  description = "Object id for pipeline runner id"
  validation {
    condition     = length(var.client_config_current_object_id) > 0
    error_message = "You must provide a value for client config current object id."
  }
}

variable "environment" {
  type        = string
  description = "Environment for resources"
  validation {
    condition     = length(var.environment) > 0
    error_message = "You must provide a value for environment."
  }
}

variable "grafana_major_version" {
  type        = number
  default     = 11
  description = "Managed Grafana major version."
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

variable "tenant_id" {
  type        = string
  description = "Tenant id for resources"
  validation {
    condition     = length(var.tenant_id) > 0
    error_message = "You must provide a value for tenant_id."
  }
}

variable "workspace_integrations" {
  type        = list(string)
  default     = []
  description = "List of azure monitor workspaces to connect grafana."
}
