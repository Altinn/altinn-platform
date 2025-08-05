output "dis_apim_workload_identity_client_id" {
  value     = azurerm_user_assigned_identity.disapim_identity.client_id
  sensitive = true
}