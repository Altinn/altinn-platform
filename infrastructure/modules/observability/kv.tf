data "azurerm_client_config" "current" {}

locals {
  # hide it from plan / apply since linters can complain
  tenant_id = sensitive(data.azurerm_client_config.current.tenant_id)
}

# Create random postfix for key vault
resource "random_string" "obs_kv_postfix" {
  length  = 6
  special = false
}

resource "azurerm_key_vault" "obs_kv" {
  name                       = substr("obs-${var.prefix}-${var.environment}-${random_string.obs_kv_postfix.result}", 0, 24)
  location                   = var.location
  resource_group_name        = azurerm_resource_group.obs.name
  sku_name                   = "standard"
  tenant_id                  = local.tenant_id
  tags                       = var.tags
  enable_rbac_authorization  = true
  purge_protection_enabled   = true
  soft_delete_retention_days = 7
}

## role
resource "azurerm_role_assignment" "obs_kv_reader" {
  scope                            = azurerm_key_vault.obs_kv.id
  role_definition_name             = "Key Vault Secrets User"
  principal_id                     = var.pipeline_sp_object_id
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

resource "azurerm_role_assignment" "ci_kv_secrets_role" {
  scope                            = azurerm_key_vault.obs_kv.id
  role_definition_name             = "Key Vault Secrets Officer" # read + write secrets only
  principal_id                     = data.azurerm_client_config.current.object_id
  skip_service_principal_aad_check = true
}
