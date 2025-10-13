# AKS-specific tags with automatic capacity calculation
# Copy this file into your AKS Terraform project

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

# Variables for AKS projects
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

# AKS-specific variables
variable "pool_configs" {
  description = "AKS node pool configurations for automatic capacity calculation"
  type = map(object({
    vm_size              = string
    auto_scaling_enabled = optional(bool, true)
    node_count           = optional(number, 1)
    min_count            = optional(number, 1)
    max_count            = number
  }))
}

# Additional capacity for other resources (optional)
variable "additional_capacity" {
  description = "Additional vCPU capacity from other resources (PostgreSQL, VMs, etc.)"
  type        = list(number)
  default     = []
  validation {
    condition = alltrue([for value in var.additional_capacity : value >= 0])
    error_message = "All additional capacity values must be non-negative numbers."
  }
}

# Local values for AKS capacity calculation
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

  # VM size to vCPU mapping for AKS node pools
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
    "standard_ds2_v2"    = 2
    "standard_ds3_v2"    = 4
    "standard_ds4_v2"    = 8
    "standard_ds5_v2"    = 16
  }

  # Calculate AKS capacity from pool configurations
  aks_capacity = sum([
    for pool_name, pool_config in var.pool_configs :
    lookup(local.vm_cpu_map, lower(pool_config.vm_size), 0) * pool_config.max_count
  ])

  # Add any additional capacity from other resources
  additional_vcpus = length(var.additional_capacity) > 0 ? sum(var.additional_capacity) : 0

  # Total capacity
  total_vcpus = local.aks_capacity + local.additional_vcpus

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
  aks_capacity_tag = {
    finops_capacity = "${local.total_vcpus}vcpu"
  }

  # Pre-built AKS tags (for convenience)
  aks_tags = merge(local.base_tags, local.aks_capacity_tag)

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

# Validation to ensure VM sizes are recognized
resource "terraform_data" "validate_vm_sizes" {
  count = length([
    for pool_name, pool_config in var.pool_configs :
    pool_name if !contains(keys(local.vm_cpu_map), lower(pool_config.vm_size))
  ]) > 0 ? 1 : 0

  lifecycle {
    precondition {
      condition = length([
        for pool_name, pool_config in var.pool_configs :
        pool_name if !contains(keys(local.vm_cpu_map), lower(pool_config.vm_size))
      ]) == 0
      error_message = <<-EOF
        Unknown VM sizes detected in pool_configs: ${join(", ", [
          for pool_name, pool_config in var.pool_configs :
          "${pool_name}: ${pool_config.vm_size}" if !contains(keys(local.vm_cpu_map), lower(pool_config.vm_size))
        ])}

        Supported VM sizes: ${join(", ", keys(local.vm_cpu_map))}
      EOF
    }
  }
}

# Outputs for debugging and verification
output "aks_capacity_calculation" {
  description = "Breakdown of AKS capacity calculation"
  value = {
    pools = {
      for pool_name, pool_config in var.pool_configs :
      pool_name => {
        vm_size     = pool_config.vm_size
        max_count   = pool_config.max_count
        vcpu_per_vm = lookup(local.vm_cpu_map, lower(pool_config.vm_size), 0)
        total_vcpu  = lookup(local.vm_cpu_map, lower(pool_config.vm_size), 0) * pool_config.max_count
      }
    }
    aks_total_vcpu      = local.aks_capacity
    additional_vcpu     = local.additional_vcpus
    grand_total_vcpu    = local.total_vcpus
    finops_capacity_tag = "${local.total_vcpus}vcpu"
  }
}

output "aks_capacity_tag" {
  description = "Capacity tag for merging with base_tags"
  value       = local.aks_capacity_tag
}

output "aks_tags" {
  description = "Tags for AKS cluster (includes finops_capacity)"
  value       = local.aks_tags
}

output "base_tags" {
  description = "Base tags for non-computing resources (no capacity)"
  value       = local.base_tags_no_capacity
}

# Usage:
#
# Option 1 - Pre-built tags:
#   tags = local.aks_tags
#
# Option 2 - Flexible merging:
#   tags = merge(local.base_tags, local.aks_capacity_tag, {
#     managed = "terraform"
#   })
#
# Non-computing resources:
#   tags = merge(local.base_tags, {
#     managed = "terraform"
#   })
#
# Example terraform.tfvars:
#   finops_environment      = "test"
#   finops_product          = "altinn2"
#   finops_serviceownercode = "skd"
#   pool_configs = {
#     syspool = {
#       vm_size              = "standard_b2s_v2"
#       max_count            = 6
#     }
#     workpool = {
#       vm_size              = "standard_b2s_v2"
#       max_count            = 6
#     }
#   }
#   additional_capacity     = [4]  # PostgreSQL: GP_Standard_D4s_v3
#   repository              = "github.com/altinn/altinn-platform"
#   current_user            = "terraform-adminservices"
#   created_date            = "2024-03-15"
