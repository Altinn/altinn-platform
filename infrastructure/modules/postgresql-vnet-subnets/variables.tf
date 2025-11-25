variable "resource_group_name" {
  type        = string
  description = "Name of the resources group where the PostgreSQL vnets and subnets are placed"
}

variable "location" {
  type        = string
  description = "Location of the PostgreSQL vnets and subnets are deployed"
}

variable "name" {
  type        = string
  description = "Name of the PostgreSQL vnet and subnets"
}

variable "vnet_address_space" {
  type        = string
  description = "IPv4 address space of the PostgreSQL vnet, must be a valid CIDR notation of size 24"

  validation {
    condition     = can(regex("^1\\.100\\.[0-9]{1,3}\\.[0-9]{1,3}/24$", var.vnet_address_space))
    error_message = "The vnet_address_space must be a valid IPv4 CIDR starting with 1.100 and must be a /24 block (e.g., 1.100.0.0/24)."
  }
}

variable "tags" {
  type        = map(string)
  description = "Set of tags to add to vnet"
  default     = {}
}

