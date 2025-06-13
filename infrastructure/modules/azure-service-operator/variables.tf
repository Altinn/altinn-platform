variable "azurerm_resource_group_aso_name" {
  description = "The name of the Azure Service Operators Resource Group."
  type        = string
  default     = ""

}

variable "azurerm_user_assigned_identity_name" {
  description = "The name of the Azure Service Operators User Assigned Managed Identity."
  type        = string
  default     = ""
}

variable "azurerm_subscription_id" {
  description = "The Azure Subscription ID where the Azure Service Operators User Assigned Managed Identity will be created."
  type        = string
}

variable "prefix" {
  description = "A prefix to be used for naming resources."
  type        = string
}

variable "environment" {
  description = "The environment for which the Azure Service Operators User Assigned Managed Identity is being created."
  type        = string
}

variable "location" {
  description = "The Azure region where the resources will be created."
  default     = "norwayeast"
  type        = string
}

variable "azurerm_kubernetes_cluster_id" {
  description = "The ID of the AKS cluster where the Azure Service Operator will be deployed."
  type        = string
}

variable "azurerm_kubernetes_cluster_oidc_issuer_url" {
  description = "The OIDC issuer URL of the AKS cluster."
  type        = string
}

variable "aso_namespace" {
  description = "The namespace where the Azure Service Operator will be deployed."
  type        = string
  default     = "azureserviceoperator-system"
}

variable "aso_service_account_name" {
  description = "The name of the service account for the Azure Service Operator."
  type        = string
  default     = "azureserviceoperator-system"
}

variable "aso_crd_pattern" {
  description = "The pattern for the Azure Service Operator Custom Resource Definitions (CRDs)."
  type        = string
  default     = "managedidentity.azure.com/*;authorization.azure.com/*"
}

variable "tags" {
  description = "A map of tags to assign to the Azure Service Operators User Assigned Managed Identity."
  type        = map(string)
  default     = {}
}

variable "flux_release_tag" {
  description = "The release tag for the Flux configuration."
  type        = string
  default     = "latest"
}

variable "dis_resource_group_id" {
  description = "The resource group ID where the Azure Service Operator resources will be created."
  type        = string
  default     = ""
  validation {
    condition     = length(var.dis_resource_group_id) > 0
    error_message = "You must provide a value for dis_resource_group_id."
  }
}
