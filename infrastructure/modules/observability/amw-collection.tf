resource "azurerm_monitor_data_collection_endpoint" "amw" {
  count               = var.azurerm_kubernetes_cluster.aks.id != "" ? 1 : 0
  name                = "${azurerm_monitor_workspace.obs.name}-mdce"
  resource_group_name = azurerm_resource_group.obs.name
  location            = azurerm_resource_group.obs.location
  kind                = "Linux"
}

resource "azurerm_monitor_data_collection_rule" "amw" {
  count                       = var.azurerm_kubernetes_cluster.aks.id != "" ? 1 : 0
  name                        = "${azurerm_monitor_workspace.obs.name}-mdcr"
  resource_group_name         = azurerm_resource_group.obs.name
  location                    = azurerm_resource_group.obs.location
  data_collection_endpoint_id = azurerm_monitor_data_collection_endpoint.amw[0].id
  kind                        = "Linux"

  destinations {
    monitor_account {
      monitor_account_id = azurerm_monitor_workspace.obs.id
      name               = azurerm_monitor_workspace.obs.name
    }
  }

  data_flow {
    streams      = ["Microsoft-PrometheusMetrics"]
    destinations = [azurerm_monitor_workspace.obs.name]
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
  count                   = var.azurerm_kubernetes_cluster.aks.id != "" ? 1 : 0
  name                    = "${azurerm_monitor_workspace.obs.name}-mdcra"
  target_resource_id      = var.azurerm_kubernetes_cluster.aks.id
  data_collection_rule_id = azurerm_monitor_data_collection_rule.amw[0].id
  description             = "Association of data collection rule. Deleting this association will break the data collection for this AKS Cluster."
  depends_on = [
    azurerm_monitor_data_collection_rule.amw
  ]
}