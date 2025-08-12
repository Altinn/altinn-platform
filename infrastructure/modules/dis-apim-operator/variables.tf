variable "kubernetes_node_resource_group" {
  type        = string
  description = "AKS node resource group name"

  validation {
    condition     = length(trim(var.kubernetes_node_resource_group)) > 0
    error_message = "kubernetes_node_resource_group cannot be empty."
  }
}

variable "kubernetes_node_location" {
  type        = string
  description = "AKS node location"

  validation {
    condition     = length(trim(var.kubernetes_node_location)) > 0
    error_message = "kubernetes_node_location cannot be empty."
  }
}

variable "kubernetes_cluster_oidc_issuer_url" {
  type        = string
  description = "The OIDC issuer URL of the AKS cluster."

  validation {
    condition     = length(trim(var.kubernetes_cluster_oidc_issuer_url)) > 0
    error_message = "kubernetes_cluster_oidc_issuer_url cannot be empty."
  }
}

variable "tags" {
  description = "A map of tags to assign to the Azure Service Operators User Assigned Managed Identity."
  type        = map(string)
  default     = {}
}

variable "user_assigned_identity_name" {
  type        = string
  description = "User assigned identity name"
  default     = ""
}

variable "kubernetes_cluster_id" {
  type        = string
  description = "AKS cluster resource id"

  validation {
    condition     = length(trim(var.kubernetes_cluster_id)) > 0
    error_message = "kubernetes_cluster_id cannot be empty."
  }
}

variable "apim_id" {
  type        = string
  description = "APIM resource id"

  validation {
    condition     = length(trim(var.apim_id)) > 0
    error_message = "apim_id cannot be empty."
  }
}

variable "apim_subscription_id" {
  type        = string
  description = "Subscription id where the APIM service is located"

  validation {
    condition     = can(regex("^[0-9a-fA-F-]{36}$", var.apim_subscription_id))
    error_message = "apim_subscription_id must be a valid UUID."
  }
}

variable "apim_resource_group_name" {
  type        = string
  description = "Resource group where the APIM service is located"

  validation {
    condition     = length(trim(var.apim_resource_group_name)) > 0
    error_message = "apim_resource_group_name cannot be empty."
  }
}

variable "apim_service_name" {
  type        = string
  description = "APIM service name"

  validation {
    condition     = length(trim(var.apim_service_name)) > 0
    error_message = "apim_service_name cannot be empty."
  }
}

variable "target_namespace" {
  type        = string
  description = "Namespace where the operator deployment will be created"

  validation {
    condition     = can(regex("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$", var.target_namespace))
    error_message = "target_namespace must be a valid Kubernetes namespace (DNS-1123 label)."
  }
}

variable "flux_release_tag" {
  type        = string
  description = "Flux release tag"
  default     = "latest"
}
