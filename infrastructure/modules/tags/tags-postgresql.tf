# PostgreSQL-specific tags with automatic SKU-based capacity calculation
# Copy this file into your PostgreSQL Terraform project

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

# Variables for PostgreSQL projects
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

# PostgreSQL-specific variables
variable "postgresql_sku" {
  description = "PostgreSQL Flexible Server SKU name for automatic capacity calculation"
  type        = string
  validation {
    condition = can(regex("^(B_Standard_B|GP_Standard_D|MO_Standard_E)", var.postgresql_sku))
    error_message = "PostgreSQL SKU must be a valid Azure PostgreSQL Flexible Server SKU (B_Standard_B*, GP_Standard_D*, MO_Standard_E*)."
  }
}

# Additional capacity for other resources (optional)
variable "additional_capacity" {
  description = "Additional vCPU capacity from other resources (AKS, VMs, etc.)"
  type        = list(number)
  default     = []
  validation {
    condition = alltrue([for value in var.additional_capacity : value >= 0])
    error_message = "All additional capacity values must be non-negative numbers."
  }
}

# Local values for PostgreSQL capacity calculation
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

  # PostgreSQL SKU to vCPU mapping
  postgresql_cpu_map = {
    # Burstable (B-series)
    "B_Standard_B1ms"  = 1
    "B_Standard_B2s"   = 2
    "B_Standard_B2ms"  = 2
    "B_Standard_B4ms"  = 4
    "B_Standard_B8ms"  = 8
    "B_Standard_B12ms" = 12
    "B_Standard_B16ms" = 16
    "B_Standard_B20ms" = 20

    # General Purpose (GP-series)
    "GP_Standard_D2s_v3"  = 2
    "GP_Standard_D4s_v3"  = 4
    "GP_Standard_D8s_v3"  = 8
    "GP_Standard_D16s_v3" = 16
    "GP_Standard_D32s_v3" = 32
    "GP_Standard_D48s_v3" = 48
    "GP_Standard_D64s_v3" = 64

    # Memory Optimized (MO-series)
    "MO_Standard_E2s_v3"  = 2
    "MO_Standard_E4s_v3"  = 4
    "MO_Standard_E8s_v3"  = 8
    "MO_Standard_E16s_v3" = 16
    "MO_Standard_E20s_v3" = 20
    "MO_Standard_E32s_v3" = 32
    "MO_Standard_E48s_v3" = 48
    "MO_Standard_E64s_v3" = 64
  }

  # Calculate PostgreSQL capacity from SKU
  postgresql_capacity = lookup(local.postgresql_cpu_map, var.postgresql_sku, 0)

  # Add any additional capacity from other resources
  additional_vcpus = length(var.additional_capacity) > 0 ? sum(var.additional_capacity) : 0

  # Total capacity
  total_vcpus = local.postgresql_capacity + local.additional_vcpus

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
  postgresql_capacity_tag = {
    finops_capacity = "${local.total_vcpus}vcpu"
  }

  # Pre-built PostgreSQL tags (for convenience)
  postgresql_tags = merge(local.base_tags, local.postgresql_capacity_tag)

  # Tags for non-computing resources (no capacity)
  base_tags_no_capacity = local.base_tags
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

# Validation to ensure PostgreSQL SKU is recognized
resource "terraform_data" "validate_postgresql_sku" {
  count = contains(keys(local.postgresql_cpu_map), var.postgresql_sku) ? 0 : 1

  lifecycle {
    precondition {
      condition = contains(keys(local.postgresql_cpu_map), var.postgresql_sku)
      error_message = <<-EOF
        Unknown PostgreSQL SKU: ${var.postgresql_sku}

        Supported SKUs:
        Burstable: ${join(", ", [for k, v in local.postgresql_cpu_map : k if can(regex("^B_", k))])}
        General Purpose: ${join(", ", [for k, v in local.postgresql_cpu_map : k if can(regex("^GP_", k))])}
        Memory Optimized: ${join(", ", [for k, v in local.postgresql_cpu_map : k if can(regex("^MO_", k))])}
      EOF
    }
  }
}

# Outputs for debugging and verification
output "postgresql_capacity_calculation" {
  description = "Breakdown of PostgreSQL capacity calculation"
  value = {
    postgresql_sku       = var.postgresql_sku
    postgresql_vcpu      = local.postgresql_capacity
    additional_vcpu      = local.additional_vcpus
    total_vcpu          = local.total_vcpus
    finops_capacity_tag = "${local.total_vcpus}vcpu"
  }
}

output "postgresql_capacity_tag" {
  description = "Capacity tag for merging with base_tags"
  value       = local.postgresql_capacity_tag
}

output "postgresql_tags" {
  description = "Tags for PostgreSQL and other computing resources (includes finops_capacity)"
  value       = local.postgresql_tags
}

output "base_tags" {
  description = "Base tags for non-computing resources (no capacity)"
  value       = local.base_tags_no_capacity
}

# Usage:
#
# Option 1 - Pre-built tags:
#   tags = local.postgresql_tags
#
# Option 2 - Flexible merging:
#   tags = merge(local.base_tags, local.postgresql_capacity_tag, {
#     managed = "terraform"
#   })
#
# Non-computing resources:
#   tags = merge(local.base_tags, {
#     managed = "terraform"
#   })
#
# Example terraform.tfvars:
#   finops_environment      = "prod"
#   finops_product          = "dialogporten"
#   finops_serviceownercode = "skd"
#   postgresql_sku          = "GP_Standard_D4s_v3"
#   additional_capacity     = [24]  # AKS cluster capacity
#   repository              = "github.com/altinn/dialogporten"
#   current_user            = "terraform-database-team"
#   created_date            = "2024-03-15"
