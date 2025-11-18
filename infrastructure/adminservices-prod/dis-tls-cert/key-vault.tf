# DIS TLS Certificate Key Vault Module
#
# This module creates an Azure Key Vault instance for managing TLS certificates
# for the DIS (Digdir Infrastructure Services) component of the Altinn Platform.
# The module uses Azure RBAC for authorization rather than access policies.

resource "azurerm_resource_group" "dis_tls_cert" {
  name     = "dis-tls-cert"
  location = var.location
  tags = merge(local.localtags, {
    submodule = "dis-tls-cert"
  })
}

resource "azurerm_key_vault" "dis_tls_cert" {
  name                            = "dis-tls-cert"
  location                        = azurerm_resource_group.dis_tls_cert.location
  resource_group_name             = azurerm_resource_group.dis_tls_cert.name
  sku_name                        = "standard"
  tenant_id                       = data.azurerm_client_config.current.tenant_id
  soft_delete_retention_days      = 90
  purge_protection_enabled        = true
  enabled_for_disk_encryption     = true
  enabled_for_template_deployment = true
  rbac_authorization_enabled      = true


  network_acls {
    bypass                     = "AzureServices"
    default_action             = "Deny"
    ip_rules                   = concat(local.github_actions_ip_ranges, var.keyvault_ip_rules)
    virtual_network_subnet_ids = var.keyvault_virtual_network_subnet_ids
  }

  tags = merge(local.localtags, {
    submodule = "dis-tls-cert"
  })
}
