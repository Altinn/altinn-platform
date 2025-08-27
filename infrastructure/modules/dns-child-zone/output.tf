output "azuread_cert_manager_client_id" {
  sensitive = true
  value     = azuread_application.cert_manager_app.client_id
}

output "azurerm_dns_zone_name" {
  value = azurerm_dns_zone.child_zone.name
}

output "azurerm_dns_zone_resource_group_name" {
  value = azurerm_dns_zone.child_zone.resource_group_name
}
