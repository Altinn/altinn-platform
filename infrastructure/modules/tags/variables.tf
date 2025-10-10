variable "capacity_values" {
  description = "List of capacity values (in vCPUs) to be summed for total finops_capacity. Only provide for computing resources (AKS, VMs, PostgreSQL, etc.)"
  type        = list(number)
  default     = []

  validation {
    condition = alltrue([
      for value in var.capacity_values : value >= 0
    ])
    error_message = "All capacity values must be non-negative numbers."
  }
}

variable "include_capacity_tag" {
  description = "Whether to include the finops_capacity tag. Set to true only for computing resources (AKS, VMs, PostgreSQL, App Services, etc.). Set to false for storage accounts, networking, and other non-computing resources."
  type        = bool
  default     = null # Will be auto-determined based on whether capacity_values is provided

  validation {
    condition     = var.include_capacity_tag == null || var.include_capacity_tag == true || var.include_capacity_tag == false
    error_message = "include_capacity_tag must be true, false, or null (auto-determine)."
  }
}

variable "current_user" {
  description = "Current user/service principal running Terraform. Used for both createdby and modifiedby."
  type        = string

  validation {
    condition     = can(regex("^[a-zA-Z0-9._@-]+$", var.current_user)) && length(var.current_user) >= 3
    error_message = "current_user must be a meaningful identity (username, service principal name, or application name) with at least 3 characters."
  }
}

variable "finops_environment" {
  description = "Environment designation for cost allocation"
  type        = string

  validation {
    condition     = can(regex("^(dev|test|prod|at22|at23|at24|yt01|tt02)$", var.finops_environment))
    error_message = "Environment must be one of: dev, test, prod, at22, at23, at24, yt01, tt02."
  }
}

variable "finops_product" {
  description = "Product name for cost allocation"
  type        = string

  validation {
    condition     = can(regex("^(studio|dialogporten|formidling|autorisasjon|varsling|melding|altinn2)$", var.finops_product))
    error_message = "Product must be one of: studio, dialogporten, formidling, autorisasjon, varsling, melding, altinn2."
  }
}

variable "finops_serviceownercode" {
  description = "Service owner code for billing attribution - will be used to lookup organization number from Altinn CDN"
  type        = string

  validation {
    condition     = can(regex("^[a-zA-Z]+$", var.finops_serviceownercode))
    error_message = "Service owner code must be letters only. Check https://altinncdn.no/orgs/altinn-orgs.json for valid codes."
  }

  # Note: Additional validation for code existence in external data is handled in locals.tf
  # to avoid circular dependencies with the data source
}

variable "finops_serviceownerorgnr" {
  description = "Service owner organization number (9 digits). If not provided, will be automatically looked up from finops_serviceownercode"
  type        = string
  default     = null

  validation {
    condition     = var.finops_serviceownerorgnr == null || can(regex("^[0-9]{9}$", var.finops_serviceownerorgnr))
    error_message = "Service owner organization number must be exactly 9 digits when provided."
  }
}

variable "repository" {
  description = "Repository URL for infrastructure as code traceability"
  type        = string

  validation {
    condition     = can(regex("^github\\.com/altinn/", var.repository))
    error_message = "Repository must be from github.com/altinn/ organization."
  }
}
