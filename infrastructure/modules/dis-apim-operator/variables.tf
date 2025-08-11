variable "azurerm_kubernetes_node_resource_group" {
  type        = string
  description = "AKS node resource group name"
}

variable "azurerm_kubernetes_node_location" {
  type        = string
  description = "AKS node location"
}

variable "azurerm_kubernetes_cluster_oidc_issuer_url" {
  type        = string
  description = "The OIDC issuer URL of the AKS cluster."
}

variable "azurerm_tags" {
  description = "A map of tags to assign to the Azure Service Operators User Assigned Managed Identity."
  type        = map(string)
  default     = {}
}

variable "azurerm_user_assigned_identity_name" {
  type        = string
  description = "User assigned identity name"
  default     = ""
}

variable "azurerm_kubernetes_cluster_id" {
  type        = string
  description = "AKS cluster resource id"
}

variable "azurerm_apim_id" {
  type        = string
  description = "APIM resource id"
}

variable "dis_apim_subscription_id" {
  type        = string
  description = "Subscription id where the APIM service is located"
}

variable "dis_apim_resource_group_name" {
  type        = string
  description = "Resource group where the APIM service is located"
}

variable "dis_apim_service_name" {
  type        = string
  description = "APIM service name"
}

variable "dis_apim_target_namespace" {
  type        = string
  description = "Namespace where the operator deployment will be created"
}

variable "flux_release_tag" {
  type        = string
  description = "Flux release tag"
  default = "latest"
}
