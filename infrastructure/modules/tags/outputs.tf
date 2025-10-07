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
  description = "Service owner organization number"
  value       = local.tags.finops_serviceownerorgnr
}

output "finops_capacity" {
  description = "Normalized capacity specification"
  value       = local.tags.finops_capacity
}

output "repository" {
  description = "Normalized repository URL"
  value       = local.tags.repository
}

output "createdby" {
  description = "Who or what created the resource"
  value       = local.tags.createdby
}

output "modifiedby" {
  description = "Who or what last modified the resource"
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
