resource "azurerm_log_analytics_workspace" "aks" {
  name                = var.azurerm_log_analytics_workspace_aks_name != "" ? var.azurerm_log_analytics_workspace_aks_name : "${var.prefix}-${var.environment}-aks-law"
  resource_group_name = azurerm_resource_group.monitor.name
  location            = azurerm_resource_group.monitor.location
  retention_in_days   = 30
  identity {
    type = "SystemAssigned"
  }
}

resource "random_id" "aks_log" {
  byte_length = 3 # 3 gives 6 characters
}

resource "azurerm_storage_account" "aks_log" {
  name                            = var.azurerm_storage_account_aks_name != "" ? var.azurerm_storage_account_aks_name : "${var.prefix}${var.environment}akslog${random_id.aks_log.hex}"
  resource_group_name             = azurerm_resource_group.monitor.name
  location                        = azurerm_resource_group.monitor.location
  account_tier                    = "Standard"
  account_replication_type        = "ZRS"
  account_kind                    = "StorageV2"
  min_tls_version                 = "TLS1_2"
  is_hns_enabled                  = true
  public_network_access_enabled   = false
  allow_nested_items_to_be_public = false
  shared_access_key_enabled       = false

  network_rules {
    default_action = "Deny"
    bypass         = ["AzureServices"]
    ip_rules = [
    ]
    virtual_network_subnet_ids = [
    ]
  }
}
