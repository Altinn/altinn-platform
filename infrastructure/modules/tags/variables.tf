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
    condition     = contains(keys(jsondecode(data.http.altinn_orgs.response_body).orgs), var.finops_serviceownercode)
    error_message = "Service owner code must exist in the Altinn organization registry. Check https://altinncdn.no/orgs/altinn-orgs.json for valid codes."
  }
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


# Data source to fetch organization data for validation
data "http" "altinn_orgs" {
  url = "https://altinncdn.no/orgs/altinn-orgs.json"
}



variable "capacity_values" {
  description = "Map of capacity values (in vCPUs) to be summed for total finops_capacity"
  type        = map(number)
  default     = {}

  validation {
    condition = alltrue([
      for name, value in var.capacity_values : value >= 0
    ])
    error_message = "All capacity values must be non-negative numbers."
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
