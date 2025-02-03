resource "azurerm_log_analytics_workspace" "aks" {
  name                = "${var.prefix}-${var.environment}-aks-law"
  resource_group_name = azurerm_resource_group.aks.name
  location            = azurerm_resource_group.aks.location
  retention_in_days   = 30
  identity {
    type = "SystemAssigned"
  }
}

resource "azurerm_monitor_workspace" "aks" {
  name                = "${var.prefix}-${var.environment}-aks-amw"
  resource_group_name = azurerm_resource_group.aks.name
  location            = azurerm_resource_group.aks.location
}

resource "random_id" "aks" {
  byte_length = 3 # 3 gives 6 characters
}

resource "azurerm_storage_account" "aks" {
  name                     = "${var.prefix}${var.environment}akslog${random_id.aks.hex}"
  resource_group_name      = azurerm_resource_group.aks.name
  location                 = azurerm_resource_group.aks.location
  account_tier             = "Standard"
  account_replication_type = "ZRS"
  account_kind             = "StorageV2"
  min_tls_version          = "TLS1_2"
  is_hns_enabled           = true

  network_rules {
    default_action = "Deny"
    bypass         = ["AzureServices"]
    ip_rules = [
    ]
    virtual_network_subnet_ids = [
    ]
  }
}

resource "azurerm_monitor_diagnostic_setting" "aks" {
  name               = "AKS-Diagnostics"
  target_resource_id = azurerm_kubernetes_cluster.aks.id
  storage_account_id = azurerm_storage_account.aks.id

  enabled_log {
    category = "kube-audit-admin"
  }

  metric {
    category = "AllMetrics"
    enabled  = false
  }
}
