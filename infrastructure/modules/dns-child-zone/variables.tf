variable "prefix" {
  type        = string
  description = "Resources prefixes"
}

variable "environment" {
  type        = string
  description = "Environment"
}

variable "cluster_ipv4_address" {
  type        = string
  description = "Cluster ipv4 address"
  validation {
    condition     = can(regex("^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$", var.cluster_ipv4_address))
    error_message = "The cluster_ipv4_address must be a valid IPv4 address."
  }
}

variable "cluster_ipv6_address" {
  type        = string
  description = "Cluster ipv6 address"
}

variable "oidc_issuer_url" {
  type        = string
  description = "Oidc issuer url needed for federation"
}

variable "location" {
  type        = string
  description = "Location for resources"
  default     = "norwayeast"
}

variable "child_dns_zone_rg_name" {
  type        = string
  description = "Override generated name for resource group for child dns zone."
  default     = ""
}

variable "parent_dns_zone_name" {
  type        = string
  description = "Parent zone name"
  default     = "altinn.cloud"
}

variable "child_dns_zone_name" {
  type        = string
  description = "Child zone name"
  default     = ""
}

variable "parent_dns_zone_rg" {
  type        = string
  description = "Resource group for parent dns zone"
  default     = "altinn-dns-zone-rg"
}
