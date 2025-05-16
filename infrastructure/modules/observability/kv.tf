data "azurerm_client_config" "current" {}

resource "azurerm_key_vault" "obs_kv" {
  lifecycle {
    prevent_destroy = true
  }
  name                = "obs-${var.prefix}-${var.environment}-kv"
  location            = var.location
  resource_group_name = azurerm_resource_group.obs.name
  sku_name            = "standard"
  tenant_id           = data.azurerm_client_config.current.tenant_id
  tags                = var.tags

}

## role
resource "azurerm_role_assignment" "obs_kv_reader" {
  scope                            = azurerm_key_vault.obs_kv.id
  role_definition_name             = "Key Vault Secrets User"
  principal_id                     = azuread_service_principal.sp.object_id
  skip_service_principal_aad_check = true
}

## add connection string to key vault
resource "azurerm_key_vault_secret" "conn_string" {
  name            = "connectionString"
  value           = azurerm_application_insights.obs.connection_string
  key_vault_id    = azurerm_key_vault.obs_kv.id
  expiration_date = timeadd(timestamp(), "8760h") # 1 year

  lifecycle {
    ignore_changes = [expiration_date] # stop perpetual updates
  }
}
