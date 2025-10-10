output "tags" {
  description = "Map of all standardized tags for resource tagging"
  value       = local.tags
}

output "finops_environment" {
  description = "Normalized environment name"
  value       = local.tags.finops_environment
}

output "finops_product" {
  description = "Normalized product name"
  value       = local.tags.finops_product
}

output "finops_serviceownercode" {
  description = "Normalized service owner code"
  value       = local.tags.finops_serviceownercode
}

output "finops_serviceownerorgnr" {
  description = "Service owner organization number (provided as input or automatically looked up from finops_serviceownercode)"
  value       = local.tags.finops_serviceownerorgnr
}

output "finops_capacity" {
  description = "Total vCPU capacity calculated from provided capacity values (only included when capacity tag is enabled)"
  value       = local.should_include_capacity ? local.tags.finops_capacity : null
}

output "repository" {
  description = "Normalized repository URL"
  value       = local.tags.repository
}

output "createdby" {
  description = "Who or what created the resource (provided by caller)"
  value       = local.tags.createdby
}

output "modifiedby" {
  description = "Who or what last modified the resource (provided by caller)"
  value       = local.tags.modifiedby
}

output "created_date" {
  description = "Date when the tags were created"
  value       = local.tags.createddate
}

output "modified_date" {
  description = "Date when the tags were last modified"
  value       = local.tags.modifieddate
}

output "total_vcpus" {
  description = "Total vCPU capacity calculated from all provided capacity values"
  value       = local.total_vcpus
}

output "capacity_breakdown" {
  description = "List of individual capacity values used in calculation"
  value       = var.capacity_values
}

output "organization_name" {
  description = "Organization name in Norwegian (looked up from finops_serviceownercode, only available when using automatic lookup)"
  value       = var.finops_serviceownerorgnr == null ? try(local.orgs_response.orgs[var.finops_serviceownercode].name.nb, null) : null
}

output "service_owner_validation" {
  description = "Debug information for service owner code validation"
  value = {
    service_owner_exists  = local.service_owner_exists
    using_manual_override = var.finops_serviceownerorgnr != null
    available_codes_count = length(keys(local.org_lookup))
    external_data_loaded  = length(keys(local.orgs_response.orgs)) > 0
    capacity_tag_included = local.should_include_capacity
  }
}

output "available_service_owner_codes" {
  description = "List of available service owner codes from Altinn CDN (for debugging)"
  value       = sort(keys(local.org_lookup))
  sensitive   = false
}

output "is_computing_resource" {
  description = "Whether this resource is tagged as a computing resource with capacity information"
  value       = local.should_include_capacity
}
