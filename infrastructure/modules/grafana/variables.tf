variable "create_resource_group" {
  type        = bool
  default     = true
  description = "Whether to create a new resource group. If false, will use an existing resource group specified by resource_group_name."
}

variable "resource_group_name" {
  type        = string
  default     = ""
  description = "Name of the resource group. When create_resource_group is true, uses this name if provided, otherwise generates 'grafana-{prefix}-{environment}-rg'. When create_resource_group is false, this is required and must be the name of an existing resource group."

  validation {
    condition     = var.create_resource_group ? true : var.resource_group_name != ""
    error_message = "When create_resource_group is false, resource_group_name must be provided."
  }
}

variable "dashboard_name" {
  type        = string
  default     = ""
  description = "Name of Grafana dashboard. If not provided, generates 'grafana-{prefix}-{environment}'."
}

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

variable "grafana_admin_access" {
  type        = list(string)
  default     = []
  description = "List of user groups to grant admin access to grafana."
}

variable "grafana_editor_access" {
  type        = list(string)
  default     = []
  description = "List of user groups to grant editor access to grafana."
}

variable "grafana_major_version" {
  type        = number
  default     = 11
  description = "Managed Grafana major version."
}

variable "grafana_monitor_reader_subscription_id" {
  type        = list(string)
  default     = []
  description = "List of subscription ids to grant reader access to grafana."
}

variable "location" {
  type        = string
  default     = "norwayeast"
  description = "Default region for resources"
}

variable "monitor_workspace_ids" {
  type        = map(string)
  default     = {}
  description = "List of azure monitor workspaces to connect grafana."
}

variable "prefix" {
  type        = string
  description = "Prefix for resource names"
  validation {
    condition     = length(var.prefix) > 0
    error_message = "You must provide a value for prefix for name generation."
  }
}

variable "localtags" {
  type        = map(string)
  description = "A map of tags to assign to the created resources."
  default     = {}
}
