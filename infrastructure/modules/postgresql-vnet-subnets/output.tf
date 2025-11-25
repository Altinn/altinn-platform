output "subnet_ids" {
  description = "The IDs of the created subnets."
  value       = [for s in azurerm_subnet.postgresql_subnets : s.id]
}

output "vnet_id" {
  description = "The ID of the created virtual network."
  value       = azurerm_virtual_network.postgresql.id
}
