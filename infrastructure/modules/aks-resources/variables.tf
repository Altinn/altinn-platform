variable "aks_node_resource_group" {
  type        = string
  description = "AKS node resource group name"
}

variable "azurerm_kubernetes_cluster_id" {
  type        = string
  description = "AKS cluster resource id"
}

variable "flux_release_tag" {
  type        = string
  default     = "latest"
  description = "Flux release tag on oci images"
}

variable "pip4_ip_address" {
  type        = string
  description = "AKS ipv4 pulic ip"
}

variable "pip6_ip_address" {
  type        = string
  description = "AKS ipv6 public ip"
}

variable "subnet_address_prefixes" {
  type        = map(list(string))
  description = "list of subnets"
}
