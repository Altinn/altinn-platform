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
  description = "Service owner code for billing attribution"
  type        = string

  validation {
    condition     = can(regex("^(skd|udir|nav|na|[a-z]{2,5})$", var.finops_serviceownercode))
    error_message = "Service owner code must be skd, udir, nav, na, or 2-5 lowercase letters."
  }
}

variable "finops_serviceownerorgnr" {
  description = "Service owner organization number (9 digits)"
  type        = string

  validation {
    condition     = can(regex("^[0-9]{9}$", var.finops_serviceownerorgnr))
    error_message = "Service owner organization number must be exactly 9 digits."
  }
}

variable "finops_capacity" {
  description = "Capacity specification (e.g., 2vcpu, 4vcpu, 8vcpu)"
  type        = string

  validation {
    condition     = can(regex("^[0-9]+vcpu$", var.finops_capacity))
    error_message = "Capacity must be in format: {number}vcpu (e.g., 2vcpu, 4vcpu, 8vcpu)."
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

variable "createdby" {
  description = "Who or what created the resource"
  type        = string
  default     = "terraform"

  validation {
    condition     = can(regex("^(terraform|azure-policy|[a-z0-9._-]+)$", var.createdby))
    error_message = "createdby must be 'terraform', 'azure-policy', or a valid username."
  }
}

variable "modifiedby" {
  description = "Who or what last modified the resource"
  type        = string
  default     = "terraform"

  validation {
    condition     = can(regex("^(terraform|azure-policy|[a-z0-9._-]+)$", var.modifiedby))
    error_message = "modifiedby must be 'terraform', 'azure-policy', or a valid username."
  }
}
