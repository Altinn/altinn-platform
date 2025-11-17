output "key_vault_name" {
  description = "Name of the Key Vault"
  value       = azurerm_key_vault.dis_tls_cert.name
}

output "key_vault_id" {
  description = "ID of the Key Vault"
  value       = azurerm_key_vault.dis_tls_cert.id
}

output "resource_group_name" {
  description = "Name of the resource group"
  value       = azurerm_resource_group.dis_tls_cert.name
}

output "resource_group_id" {
  description = "ID of the resource group"
  value       = azurerm_resource_group.dis_tls_cert.id
}

output "key_vault_uri" {
  description = "URI of the Key Vault"
  value       = azurerm_key_vault.dis_tls_cert.vault_uri
}

output "key_vault_role_assignments" {
  description = "RBAC role assignments applied to the Key Vault"
  value = {
    terraform_admin = {
      role_definition_name = "Key Vault Administrator"
      principal_id         = data.azurerm_client_config.current.object_id
    }
    additional = var.azure_keyvault_additional_role_assignments
  }
}
