# Altinn Platform - Standardized Resource Tags
# Copy this file into your Terraform project for consistent tagging
# Data source to fetch organization data from Altinn CDN
# Only fetch when serviceownercode is provided but orgnr is not explicitly provided
data "http" "altinn_orgs" {
  count              = var.finops_serviceownercode != "" && var.finops_serviceownerorgnr == "" ? 1 : 0
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
    condition     = contains(["dev", "test", "prod", "at22", "at23", "at24", "yt01", "tt02"], var.finops_environment)
    error_message = "Environment must be one of: dev, test, prod, at22, at23, at24, yt01, tt02."
  }
}
variable "finops_product" {
  description = "Product name for cost allocation"
  type        = string
  validation {
    condition     = contains(["studio", "dialogporten", "formidling", "autorisasjon", "varsling", "melding", "altinn2"], var.finops_product)
    error_message = "Product must be one of: studio, dialogporten, formidling, autorisasjon, varsling, melding, altinn2."
  }
}
variable "finops_serviceownercode" {
  description = "Service owner code for billing attribution"
  type        = string
  default     = ""
  validation {
    condition     = var.finops_serviceownercode == "" || can(regex("^[A-Za-z]+$", var.finops_serviceownercode))
    error_message = "Service owner code must be letters only. Check https://altinncdn.no/orgs/altinn-orgs.json for valid codes."
  }
}
variable "finops_serviceownerorgnr" {
  description = "Service owner organization number. Leave empty to auto-lookup from Altinn registry using serviceownercode."
  type        = string
  default     = ""
  validation {
    condition     = var.finops_serviceownerorgnr == "" || can(regex("^[0-9]{9}$", var.finops_serviceownerorgnr))
    error_message = "Organization number must be 9 digits when provided, or empty to auto-lookup."
  }
}
variable "repository" {
  description = "Repository URL for infrastructure traceability"
  type        = string
  default     = ""
}


variable "current_user" {
  description = "Current user or service principal running Terraform. Leave empty to use Azure client config automatically."
  type        = string
  default     = ""
  validation {
    condition     = var.current_user == "" || length(var.current_user) >= 3
    error_message = "Current user must be at least 3 characters long when provided, or empty to use Azure client config."
  }
}
variable "created_date" {
  description = "Date when resources were created (YYYY-MM-DD). Leave empty to use current date (stable via time_static)"
  type        = string
  default     = ""
  validation {
    condition     = var.created_date == "" || can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}$", var.created_date))
    error_message = "Date must be in YYYY-MM-DD format when provided."
  }
}
variable "modified_date" {
  description = "Date when resources were last modified (YYYY-MM-DD). Leave empty to use current date on each run"
  type        = string
  default     = ""
  validation {
    condition     = var.modified_date == "" || can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}$", var.modified_date))
    error_message = "Date must be in YYYY-MM-DD format when provided."
  }
}
# Static time resource to prevent creation_date drift
resource "time_static" "tags_created" {}
# Static creator resource to prevent createdby drift
# This captures the current user on first run and preserves it
resource "terraform_data" "tags_creator" {
  input = var.current_user != "" ? var.current_user : data.azurerm_client_config.current.object_id
  lifecycle {
    ignore_changes = [input]
  }
}

# Data source to get current Azure client configuration
data "azurerm_client_config" "current" {}

# Local values for tag generation
locals {
  # Parse organization data with error handling
  orgs = try(jsondecode(data.http.altinn_orgs[0].response_body).orgs, {})
  # Create lookup map from service owner code to organization number
  org_lookup = {
    for code, org in local.orgs :
    lower(code) => org.orgnr
  }
  # Validate that the service owner code exists (skip if empty or if orgnr is explicitly provided)
  service_owner_exists = var.finops_serviceownerorgnr != "" ? true : (var.finops_serviceownercode == "" ? true : contains(keys(local.org_lookup), lower(var.finops_serviceownercode)))
  # Get current date if not provided
  current_date = formatdate("YYYY-MM-DD", time_static.tags_created.rfc3339)
  # Use provided dates or fallback to current date
  creation_date     = var.created_date != "" ? var.created_date : local.current_date
  modification_date = var.modified_date != "" ? var.modified_date : formatdate("YYYY-MM-DD", timestamp())
  # Use provided user or fallback to Azure client config for modifications
  current_modifier = var.current_user != "" ? var.current_user : data.azurerm_client_config.current.object_id
  # Preserve original creator from static resource
  # This ensures createdby doesn't change on subsequent runs
  original_creator = terraform_data.tags_creator.input
  # Base tags for all resources
  base_tags = {
    finops_environment       = lower(var.finops_environment)
    finops_product           = lower(var.finops_product)
    finops_serviceownercode  = var.finops_serviceownercode == "" ? "" : lower(var.finops_serviceownercode)
    finops_serviceownerorgnr = var.finops_serviceownerorgnr != "" ? var.finops_serviceownerorgnr : (var.finops_serviceownercode == "" ? "" : lookup(local.org_lookup, lower(var.finops_serviceownercode), ""))
    createdby                = lower(local.original_creator)
    createddate              = local.creation_date
    modifiedby               = lower(local.current_modifier)
    modifieddate             = local.modification_date
    repository               = var.repository == "" ? "" : lower(var.repository)
  }
}
# Validation to ensure service owner code is valid
resource "terraform_data" "validate_service_owner" {
  count = local.service_owner_exists ? 0 : 1
  lifecycle {
    precondition {
      condition     = local.service_owner_exists
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
#   tags = local.base_tags
#
# Option 2 - Flexible merging:
#   tags = merge(local.base_tags, {
#     managed = "terraform"
#   })
#
# Example terraform.tfvars:
#   finops_environment      = "prod"
#   finops_product          = "dialogporten"
#   finops_serviceownercode = "skd"
#   repository              = "github.com/altinn/dialogporten"
#   current_user            = "terraform-sp"
#   created_date            = "2024-03-15"
#   modified_date           = ""
