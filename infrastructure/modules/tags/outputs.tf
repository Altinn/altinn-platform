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
  description = "Total vCPU capacity calculated from provided capacity values"
  value       = local.tags.finops_capacity
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
  description = "Breakdown of individual capacity values used in calculation"
  value       = var.capacity_values
}

output "organization_name" {
  description = "Organization name in Norwegian (looked up from finops_serviceownercode, only available when using automatic lookup)"
  value       = var.finops_serviceownerorgnr == null ? local.orgs_response.orgs[var.finops_serviceownercode].name.nb : null
}
