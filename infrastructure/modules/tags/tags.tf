# Altinn Platform - Standardized Resource Tags
# Copy this file into your Terraform project for consistent tagging

# Data source to fetch organization data from Altinn CDN
data "http" "altinn_orgs" {
  url                = "https://altinncdn.no/orgs/altinn-orgs.json"
  request_timeout_ms = 30000

  retry {
    attempts     = 3
    min_delay_ms = 1000
    max_delay_ms = 5000
  }
}

# Variables - add these to your terraform.tfvars
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
    condition     = can(regex("^[a-zA-Z]+$", var.finops_serviceownercode))
    error_message = "Service owner code must be letters only. Check https://altinncdn.no/orgs/altinn-orgs.json for valid codes."
  }
}

variable "capacity_values" {
  description = "List of vCPU capacity values for computing resources"
  type        = list(number)
  default     = []
  validation {
    condition = alltrue([for value in var.capacity_values : value >= 0])
    error_message = "All capacity values must be non-negative numbers."
  }
}

variable "repository" {
  description = "Repository URL for infrastructure traceability"
  type        = string
  validation {
    condition     = can(regex("^github\\.com/altinn/", var.repository))
    error_message = "Repository must be from github.com/altinn/ organization."
  }
}

variable "current_user" {
  description = "Current user or service principal running Terraform"
  type        = string
  validation {
    condition     = length(var.current_user) >= 3
    error_message = "Current user must be at least 3 characters long."
  }
}

variable "created_date" {
  description = "Date when resources were created (YYYY-MM-DD). Leave empty to use current date"
  type        = string
  default     = ""
  validation {
    condition     = var.created_date == "" || can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}$", var.created_date))
    error_message = "Date must be in YYYY-MM-DD format when provided."
  }
}

variable "modified_date" {
  description = "Date when resources were last modified (YYYY-MM-DD). Leave empty to use current date"
  type        = string
  default     = ""
  validation {
    condition     = var.modified_date == "" || can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}$", var.modified_date))
    error_message = "Date must be in YYYY-MM-DD format when provided."
  }
}

# Local values for tag generation
locals {
  # Parse organization data with error handling
  orgs_response = can(jsondecode(data.http.altinn_orgs.response_body)) ?
    jsondecode(data.http.altinn_orgs.response_body) : { orgs = {} }

  # Create lookup map from service owner code to organization number
  org_lookup = {
    for code, org in local.orgs_response.orgs :
    code => org.orgnr
  }

  # Validate that the service owner code exists
  service_owner_exists = contains(keys(local.org_lookup), lower(var.finops_serviceownercode))

  # Get current date if not provided
  current_date = formatdate("YYYY-MM-DD", timestamp())

  # Use provided dates or fallback to current date
  creation_date     = var.created_date != "" ? var.created_date : local.current_date
  modification_date = var.modified_date != "" ? var.modified_date : local.current_date

  # Calculate total capacity from provided values
  total_vcpus = length(var.capacity_values) > 0 ? sum(var.capacity_values) : 0

  # SKU to vCPU mapping tables
  vm_cpu_map = {
    "standard_b1s"       = 1
    "standard_b2s"       = 2
    "standard_b2s_v2"    = 2
    "standard_b4s_v2"    = 4
    "standard_d2s_v3"    = 2
    "standard_d4s_v3"    = 4
    "standard_d8s_v3"    = 8
    "standard_d16s_v3"   = 16
    "standard_d32s_v3"   = 32
  }

  postgresql_cpu_map = {
    "GP_Standard_D2s_v3"  = 2
    "GP_Standard_D4s_v3"  = 4
    "GP_Standard_D8s_v3"  = 8
    "GP_Standard_D16s_v3" = 16
    "GP_Standard_D32s_v3" = 32
  }

  app_service_cpu_map = {
    "P1v2" = 1
    "P2v2" = 2
    "P3v2" = 4
    "S1"   = 1
    "S2"   = 2
    "S3"   = 4
  }

  # Base tags for all resources (no capacity)
  base_tags = {
    finops_environment       = lower(var.finops_environment)
    finops_product           = lower(var.finops_product)
    finops_serviceownercode  = lower(var.finops_serviceownercode)
    finops_serviceownerorgnr = local.service_owner_exists ? local.org_lookup[lower(var.finops_serviceownercode)] : ""
    createdby                = lower(var.current_user)
    createddate              = local.creation_date
    modifiedby               = lower(var.current_user)
    modifieddate             = local.modification_date
    repository               = lower(var.repository)
  }

  # Capacity tag for merging with base_tags
  capacity_tag = {
    finops_capacity = "${local.total_vcpus}vcpu"
  }

  # Pre-built capacity tags (for convenience)
  base_tags_with_capacity = merge(local.base_tags, local.capacity_tag)
}

# Validation to ensure service owner code is valid
resource "terraform_data" "validate_service_owner" {
  count = local.service_owner_exists ? 0 : 1

  lifecycle {
    precondition {
      condition = local.service_owner_exists
      error_message = <<-EOF
        Service owner code '${var.finops_serviceownercode}' not found in Altinn organization registry.
        Check https://altinncdn.no/orgs/altinn-orgs.json for valid codes.
      EOF
    }
  }
}

# Usage:
#
# Option 1 - Pre-built tags:
#   tags = local.base_tags_with_capacity
#   tags = local.base_tags
#
# Option 2 - Flexible merging:
#   tags = merge(local.base_tags, local.capacity_tag, {
#     managed = "terraform"
#   })
#   tags = merge(local.base_tags, {
#     managed = "terraform"
#   })
#
# Example terraform.tfvars:
#   finops_environment      = "prod"
#   finops_product          = "dialogporten"
#   finops_serviceownercode = "skd"
#   capacity_values         = [32, 8, 4]
#   repository              = "github.com/altinn/dialogporten"
#   current_user            = "terraform-sp"
#   created_date            = "2024-03-15"
#   modified_date           = ""
