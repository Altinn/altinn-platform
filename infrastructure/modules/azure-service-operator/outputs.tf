output "azurerm_user_assigned_identity_principal_id" {
  description = "The principal ID of the Azure Service Operator User Assigned Managed Identity."
  value       = azurerm_user_assigned_identity.aso_identity.principal_id
}

output "azurerm_resource_group_name" {
  description = "The name of the resource group where the Azure Service Operator resources are created."
  value       = azurerm_resource_group.aso_rg.name
}