variable "resource_group_name" {
  type        = string
  description = "Name of the resources group where the PostgreSQL vnets and subnets are placed"
}

variable "location" {
  type        = string
  description = "Location where the PostgreSQL vnets and subnets are deployed"
}

variable "name" {
  type        = string
  description = "Name of the PostgreSQL vnet and subnets"
}

variable "environment" {
  type        = string
  description = "Environment for resources"
  validation {
    condition     = length(trimspace(var.environment)) > 0
    error_message = "You must provide a value for environment."
  }
}

variable "oidc_issuer_url" {
  type        = string
  description = "Oidc issuer url needed for federation"
  validation {
    condition     = length(trimspace(var.oidc_issuer_url)) > 0
    error_message = "You must provide a value for oidc_issuer_url."
  }
}

variable "vnet_address_space" {
  type        = string
  description = "IPv4 address space of the PostgreSQL vnet, must be a valid CIDR notation of size 24"

  validation {
    condition     = can(cidrhost(var.vnet_address_space, 0)) && startswith(var.vnet_address_space, "10.100.") && endswith(var.vnet_address_space, "/24")
    error_message = "The vnet_address_space must be a valid IPv4 CIDR starting with 10.100 and must be a /24 block (e.g., 10.100.0.0/24)."
  }
}

variable "tags" {
  type        = map(string)
  description = "Set of tags to add to vnet"
  default     = {}
}

variable "peered_vnets" {
  type = object({
    name                = string
    id                  = string
    resource_group_name = string
  })
  description = "ID of the vnet this Vnet should be peered with"
}

variable "user_assigned_identity_name" {
  type    = string
  default = ""
}
