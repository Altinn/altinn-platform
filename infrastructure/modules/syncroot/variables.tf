variable "azurerm_kubernetes_cluster_id" {
  description = "The ID of the Azure Kubernetes Cluster."
  type        = string
  validation {
    condition     = length(var.azurerm_kubernetes_cluster_id) > 0
    error_message = "You must provide a kubernetes cluster ID."
  }
}

variable "team_name" {
  type        = string
  default     = ""
  description = "Name of the team owning the syncroot"
  validation {
    condition     = length(var.prefix) > 0
    error_message = "You must provide a value for team_name."
  }
}

variable "environment" {
  type        = string
  description = "Environment"
  validation {
    condition     = length(var.environment) > 0
    error_message = "You must provide a value for environment."
  }
}

variable "namespace" {
  type        = string
  default     = ""
  description = "The namespace to use for the syncroot. Default to value of team_name."
}
