# RBAC role assignments for Key Vault

# Assign Key Vault Administrator role to the Terraform service principal
resource "azurerm_role_assignment" "terraform_key_vault_admin" {
  scope                = azurerm_key_vault.dis_tls_cert.id
  role_definition_name = "Key Vault Administrator"
  principal_id         = data.azurerm_client_config.current.object_id
}

# Dynamic block for additional role assignments
resource "azurerm_role_assignment" "additional" {
  for_each = {
    for idx, assignment in var.azure_keyvault_additional_role_assignments :
    idx => assignment
  }
  scope                = azurerm_key_vault.dis_tls_cert.id
  role_definition_name = each.value.role_definition_name
  principal_id         = each.value.principal_id
}
