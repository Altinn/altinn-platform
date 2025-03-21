resource "azurerm_monitor_data_collection_endpoint" "amw" {
  name                = "${azurerm_monitor_workspace.aks.name}-mdce"
  resource_group_name = azurerm_resource_group.monitor.name
  location            = azurerm_resource_group.monitor.location
  kind                = "Linux"
}

resource "azurerm_monitor_data_collection_rule" "amw" {
  name                        = "${azurerm_monitor_workspace.aks.name}-mdcr"
  resource_group_name         = azurerm_resource_group.monitor.name
  location                    = azurerm_resource_group.monitor.location
  data_collection_endpoint_id = azurerm_monitor_data_collection_endpoint.amw.id
  kind                        = "Linux"

  destinations {
    monitor_account {
      monitor_account_id = azurerm_monitor_workspace.aks.id
      name               = azurerm_monitor_workspace.aks.name
    }
  }

  data_flow {
    streams      = ["Microsoft-PrometheusMetrics"]
    destinations = ["${azurerm_monitor_workspace.aks.name}"]
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
  name                    = "${azurerm_monitor_workspace.aks.name}-mdcra"
  target_resource_id      = azurerm_kubernetes_cluster.aks.id
  data_collection_rule_id = azurerm_monitor_data_collection_rule.amw.id
  description             = "Association of data collection rule. Deleting this association will break the data collection for this AKS Cluster."
  depends_on = [
    azurerm_monitor_data_collection_rule.amw
  ]
}
