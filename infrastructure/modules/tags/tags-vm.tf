# VM-specific tags with automatic capacity calculation from VM configurations
# Copy this file into your VM Terraform project

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

# Variables for VM projects
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

# VM-specific variables
variable "vm_configs" {
  description = "VM configurations for automatic capacity calculation"
  type = map(object({
    vm_size = string
    count   = optional(number, 1)
  }))
}

# Additional capacity for other resources (optional)
variable "additional_capacity" {
  description = "Additional vCPU capacity from other resources (AKS, PostgreSQL, etc.)"
  type        = list(number)
  default     = []
  validation {
    condition = alltrue([for value in var.additional_capacity : value >= 0])
    error_message = "All additional capacity values must be non-negative numbers."
  }
}

# Local values for VM capacity calculation
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

  # VM size to vCPU mapping
  vm_cpu_map = {
    # B-series (Burstable)
    "standard_b1ls"      = 1
    "standard_b1ms"      = 1
    "standard_b1s"       = 1
    "standard_b2ms"      = 2
    "standard_b2s"       = 2
    "standard_b4ms"      = 4
    "standard_b8ms"      = 8
    "standard_b12ms"     = 12
    "standard_b16ms"     = 16
    "standard_b20ms"     = 20

    # D-series v2 (General Purpose)
    "standard_d1_v2"     = 1
    "standard_d2_v2"     = 2
    "standard_d3_v2"     = 4
    "standard_d4_v2"     = 8
    "standard_d5_v2"     = 16
    "standard_d11_v2"    = 2
    "standard_d12_v2"    = 4
    "standard_d13_v2"    = 8
    "standard_d14_v2"    = 16

    # DS-series v2 (General Purpose with Premium Storage)
    "standard_ds1_v2"    = 1
    "standard_ds2_v2"    = 2
    "standard_ds3_v2"    = 4
    "standard_ds4_v2"    = 8
    "standard_ds5_v2"    = 16
    "standard_ds11_v2"   = 2
    "standard_ds12_v2"   = 4
    "standard_ds13_v2"   = 8
    "standard_ds14_v2"   = 16

    # D-series v3 (General Purpose)
    "standard_d2_v3"     = 2
    "standard_d4_v3"     = 4
    "standard_d8_v3"     = 8
    "standard_d16_v3"    = 16
    "standard_d32_v3"    = 32
    "standard_d48_v3"    = 48
    "standard_d64_v3"    = 64

    # DS-series v3 (General Purpose with Premium Storage)
    "standard_d2s_v3"    = 2
    "standard_d4s_v3"    = 4
    "standard_d8s_v3"    = 8
    "standard_d16s_v3"   = 16
    "standard_d32s_v3"   = 32
    "standard_d48s_v3"   = 48
    "standard_d64s_v3"   = 64

    # E-series v3 (Memory Optimized)
    "standard_e2_v3"     = 2
    "standard_e4_v3"     = 4
    "standard_e8_v3"     = 8
    "standard_e16_v3"    = 16
    "standard_e20_v3"    = 20
    "standard_e32_v3"    = 32
    "standard_e48_v3"    = 48
    "standard_e64_v3"    = 64

    # ES-series v3 (Memory Optimized with Premium Storage)
    "standard_e2s_v3"    = 2
    "standard_e4s_v3"    = 4
    "standard_e8s_v3"    = 8
    "standard_e16s_v3"   = 16
    "standard_e20s_v3"   = 20
    "standard_e32s_v3"   = 32
    "standard_e48s_v3"   = 48
    "standard_e64s_v3"   = 64

    # F-series v2 (Compute Optimized)
    "standard_f2s_v2"    = 2
    "standard_f4s_v2"    = 4
    "standard_f8s_v2"    = 8
    "standard_f16s_v2"   = 16
    "standard_f32s_v2"   = 32
    "standard_f48s_v2"   = 48
    "standard_f64s_v2"   = 64
    "standard_f72s_v2"   = 72

    # B-series v2 (Burstable)
    "standard_b2s_v2"    = 2
    "standard_b4s_v2"    = 4
    "standard_b8s_v2"    = 8
    "standard_b16s_v2"   = 16
    "standard_b32s_v2"   = 32

    # Legacy A-series
    "standard_a0"        = 1
    "standard_a1"        = 1
    "standard_a2"        = 2
    "standard_a3"        = 4
    "standard_a4"        = 8
  }

  # Calculate VM capacity from configurations
  vm_capacity = sum([
    for vm_name, vm_config in var.vm_configs :
    lookup(local.vm_cpu_map, lower(vm_config.vm_size), 0) * vm_config.count
  ])

  # Add any additional capacity from other resources
  additional_vcpus = length(var.additional_capacity) > 0 ? sum(var.additional_capacity) : 0

  # Total capacity
  total_vcpus = local.vm_capacity + local.additional_vcpus

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
  vm_capacity_tag = {
    finops_capacity = "${local.total_vcpus}vcpu"
  }

  # Pre-built VM tags (for convenience)
  vm_tags = merge(local.base_tags, local.vm_capacity_tag)

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
    for vm_name, vm_config in var.vm_configs :
    vm_name if !contains(keys(local.vm_cpu_map), lower(vm_config.vm_size))
  ]) > 0 ? 1 : 0

  lifecycle {
    precondition {
      condition = length([
        for vm_name, vm_config in var.vm_configs :
        vm_name if !contains(keys(local.vm_cpu_map), lower(vm_config.vm_size))
      ]) == 0
      error_message = <<-EOF
        Unknown VM sizes detected in vm_configs: ${join(", ", [
          for vm_name, vm_config in var.vm_configs :
          "${vm_name}: ${vm_config.vm_size}" if !contains(keys(local.vm_cpu_map), lower(vm_config.vm_size))
        ])}

        Supported VM sizes: ${join(", ", sort(keys(local.vm_cpu_map)))}
      EOF
    }
  }
}

# Outputs for debugging and verification
output "vm_capacity_calculation" {
  description = "Breakdown of VM capacity calculation"
  value = {
    vms = {
      for vm_name, vm_config in var.vm_configs :
      vm_name => {
        vm_size     = vm_config.vm_size
        count       = vm_config.count
        vcpu_per_vm = lookup(local.vm_cpu_map, lower(vm_config.vm_size), 0)
        total_vcpu  = lookup(local.vm_cpu_map, lower(vm_config.vm_size), 0) * vm_config.count
      }
    }
    vm_total_vcpu       = local.vm_capacity
    additional_vcpu     = local.additional_vcpus
    grand_total_vcpu    = local.total_vcpus
    finops_capacity_tag = "${local.total_vcpus}vcpu"
  }
}

output "vm_capacity_tag" {
  description = "Capacity tag for merging with base_tags"
  value       = local.vm_capacity_tag
}

output "vm_tags" {
  description = "Tags for VMs and other computing resources (includes finops_capacity)"
  value       = local.vm_tags
}

output "base_tags" {
  description = "Base tags for non-computing resources (no capacity)"
  value       = local.base_tags_no_capacity
}

# Usage:
#
# Option 1 - Pre-built tags:
#   tags = local.vm_tags
#
# Option 2 - Flexible merging:
#   tags = merge(local.base_tags, local.vm_capacity_tag, {
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
#   finops_product          = "studio"
#   finops_serviceownercode = "skd"
#   vm_configs = {
#     web_server = {
#       vm_size = "standard_d4s_v3"
#       count   = 2
#     }
#     app_server = {
#       vm_size = "standard_d8s_v3"
#       count   = 3
#     }
#   }
#   additional_capacity     = [8]  # PostgreSQL: GP_Standard_D8s_v3
#   repository              = "github.com/altinn/altinn-studio"
#   current_user            = "terraform-vm-team"
#   created_date            = "2024-03-15"
