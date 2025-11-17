variable "location" {
  description = "Azure region where resources will be deployed. Example: 'norwayeast'"
  type        = string

  validation {
    condition     = can(regex("^[a-z]+[a-z0-9]*$", var.location))
    error_message = "The location must be a valid Azure region name (lowercase, alphanumeric)."
  }
}

variable "subscription_id" {
  description = "Azure subscription ID where resources will be deployed"
  type        = string

  validation {
    condition     = can(regex("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", var.subscription_id))
    error_message = "The subscription_id must be a valid GUID format."
  }
}

variable "tags" {
  description = "Additional tags to apply to all resources (currently not used in merge)"
  type        = map(string)
  default     = {}
}

variable "azure_keyvault_additional_role_assignments" {
  description = "Additional RBAC role assignments for the Key Vault beyond the Terraform service principal"
  type = list(object({
    role_definition_name = string
    principal_id         = string
  }))
  default = []

  validation {
    condition = alltrue([
      for assignment in var.azure_keyvault_additional_role_assignments :
      contains([
        "Key Vault Administrator",
        "Key Vault Certificates Officer",
        "Key Vault Crypto Officer",
        "Key Vault Crypto Service Encryption User",
        "Key Vault Crypto User",
        "Key Vault Reader",
        "Key Vault Secrets Officer",
        "Key Vault Secrets User"
      ], assignment.role_definition_name)
    ])
    error_message = "Role definition name must be a valid Key Vault RBAC role."
  }
}

variable "keyvault_ip_rules" {
  description = "List of IP addresses or CIDR ranges that are allowed to access the Key Vault"
  type        = list(string)
  default     = []
}

variable "keyvault_virtual_network_subnet_ids" {
  description = "List of virtual network subnet IDs that are allowed to access the Key Vault"
  type        = list(string)
  default     = []
}
