variable "environment" {
  type        = string
  description = "Environment for resources"
  validation {
    condition     = length(var.environment) > 0
    error_message = "You must provide a value for environment."
  }
}

variable "grafana_admin_sp_object_id" {
  type        = string
  description = "Object id for service pricipal to give admin access to grafana"
  validation {
    condition     = length(var.prefix) > 0
    error_message = "You must provide a value for grafana_admin_sp_object_id."
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
