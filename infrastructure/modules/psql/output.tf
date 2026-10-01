output "psql_server_name" {
  description = "The name of the PostgreSQL Flexible Server"
  value       = azurerm_postgresql_flexible_server.psql.name
}

output "psql_server_fqdn" {
  description = "The fully qualified domain name of the PostgreSQL Flexible Server"
  value       = azurerm_postgresql_flexible_server.psql.fqdn
}

output "psql_server_id" {
  description = "The ID of the PostgreSQL Flexible Server"
  value       = azurerm_postgresql_flexible_server.psql.id
}

output "psql_identity_id" {
  description = "The ID of the User Assigned Managed Identity"
  value       = azurerm_user_assigned_identity.psql_identity.id
  sensitive   = false
}

output "psql_admin_group_object_ids" {
  description = "The object IDs of the Azure AD groups used as administrators"
  value       = values(data.azuread_group.psql_admin_groups)[*].object_id
  sensitive   = false
}

output "psql_database_name" {
  description = "The name of the PostgreSQL database"
  value       = azurerm_postgresql_flexible_server_database.psql.name
}

output "psql_private_dns_zone_name" {
  description = "Name of private DNS zone if module created it; null if external supplied."
  value       = local.create_private_dns_zone ? try(azurerm_private_dns_zone.psql[0].name, null) : null
}

output "psql_effective_private_dns_zone_id" {
  description = "Effective private DNS zone ID (created or supplied). Null if private DNS is not in use."
  value       = var.psql_enable_private_access ? coalesce(
    var.existing_private_dns_zone_id,
    try(azurerm_private_dns_zone.psql[0].id, null)
  ) : null
}

output "psql_actual_storage_mb" {
  description = "Observed storage size in MB (may be > initial if AutoGrow)."
  value       = local.psql_actual_storage_mb
  depends_on  = [azurerm_postgresql_flexible_server.psql]
}
