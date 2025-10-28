resource "azurerm_monitor_data_collection_endpoint" "amw" {
  count               = var.enable_aks_monitoring ? 1 : 0
  name                = "${try(azurerm_monitor_workspace.obs[0].name, data.azurerm_monitor_workspace.existing[0].name)}-mdce"
  resource_group_name = try(azurerm_resource_group.obs[0].name, var.azurerm_resource_group_obs_name)
  location            = var.location
  kind                = "Linux"
}

resource "azurerm_monitor_data_collection_rule" "amw" {
  count                       = var.enable_aks_monitoring ? 1 : 0
  name                        = "${try(azurerm_monitor_workspace.obs[0].name, data.azurerm_monitor_workspace.existing[0].name)}-mdcr"
  resource_group_name         = try(azurerm_resource_group.obs[0].name, var.azurerm_resource_group_obs_name)
  location                    = var.location
  data_collection_endpoint_id = azurerm_monitor_data_collection_endpoint.amw[0].id
  kind                        = "Linux"

  destinations {
    monitor_account {
      monitor_account_id = try(azurerm_monitor_workspace.obs[0].id, data.azurerm_monitor_workspace.existing[0].id)
      name               = try(azurerm_monitor_workspace.obs[0].name, data.azurerm_monitor_workspace.existing[0].name)
    }
  }

  data_flow {
    streams      = ["Microsoft-PrometheusMetrics"]
    destinations = [try(azurerm_monitor_workspace.obs[0].name, data.azurerm_monitor_workspace.existing[0].name)]
  }


  data_sources {
    prometheus_forwarder {
      streams = ["Microsoft-PrometheusMetrics"]
      name    = "PrometheusDataSource"
    }
  }

  description = "DCR for Azure Monitor Metrics Profile (Managed Prometheus)"
  depends_on = [
    azurerm_monitor_data_collection_endpoint.amw
  ]
}

resource "azurerm_monitor_data_collection_rule_association" "amw" {
  count                   = var.enable_aks_monitoring ? 1 : 0
  name                    = "${try(azurerm_monitor_workspace.obs[0].name, data.azurerm_monitor_workspace.existing[0].name)}-mdcra"
  target_resource_id      = var.azurerm_kubernetes_cluster_id
  data_collection_rule_id = azurerm_monitor_data_collection_rule.amw[0].id
  description             = "Association of data collection rule. Deleting this association will break the data collection for this AKS Cluster."
  depends_on = [
    azurerm_monitor_data_collection_rule.amw
  ]
}
