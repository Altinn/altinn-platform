resource "azurerm_monitor_data_collection_endpoint" "amw" {
  count               = var.enable_aks_monitoring ? 1 : 0
  name                = "${local.amw.name}-mdce"
  resource_group_name = local.rg.name
  location            = local.rg.location
  kind                = "Linux"
  tags = merge(var.localtags, {
    submodule = "observability"
  })
}

resource "azurerm_monitor_data_collection_rule" "amw" {
  count                       = var.enable_aks_monitoring ? 1 : 0
  name                        = "${local.amw.name}-mdcr"
  resource_group_name         = local.rg.name
  location                    = local.rg.location
  data_collection_endpoint_id = azurerm_monitor_data_collection_endpoint.amw[0].id
  kind                        = "Linux"
  tags = merge(var.localtags, {
    submodule = "observability"
  })

  destinations {
    monitor_account {
      monitor_account_id = local.amw.id
      name               = local.amw.name
    }
  }

  data_flow {
    streams      = ["Microsoft-PrometheusMetrics"]
    destinations = [local.amw.name]
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
  name                    = "${local.amw.name}-mdcra"
  target_resource_id      = var.azurerm_kubernetes_cluster_id
  data_collection_rule_id = azurerm_monitor_data_collection_rule.amw[0].id
  description             = "Association of data collection rule. Deleting this association will break the data collection for this AKS Cluster."
  depends_on = [
    azurerm_monitor_data_collection_rule.amw
  ]
}
