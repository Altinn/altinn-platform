variable "resource_group_name" {
  type        = string
  description = "Name of the resource group to deploy runners into"
}

variable "repository_name" {
  type        = string
  description = "Name of the GitHub repository"
}

variable "private_runners_address_space" {
  type        = string
  description = "Address space for the private runners VNet and subnet (CIDR notation)"
  default     = ""
}

variable "private_runners_prefix" {
  type        = string
  description = "Resource name prefix for private runner resources"
  default     = ""
}

variable "altinn_app_id" {
  type        = string
  description = "GitHub App ID for Altinn"
}

variable "altinn_app_install_id" {
  type        = string
  description = "GitHub App installation ID for Altinn"
}

variable "altinn_app_key" {
  type        = string
  description = "GitHub App private key for Altinn"
  sensitive   = true
}

variable "host_ip" {
  type        = string
  description = "Host IP address for Key Vault IP rules"
}

variable "tags" {
  type        = map(string)
  description = "Tags to apply to all resources"
  default     = {}
}
