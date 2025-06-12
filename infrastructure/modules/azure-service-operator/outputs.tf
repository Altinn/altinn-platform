output "azurerm_user_assigned_identity_principal_id" {
  description = "The principal ID of the Azure Service Operator User Assigned Managed Identity."
  value       = azurerm_user_assigned_identity.aso_identity.principal_id
}
