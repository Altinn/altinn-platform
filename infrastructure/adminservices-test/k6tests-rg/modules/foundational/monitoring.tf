resource "azurerm_monitor_workspace" "k6tests" {
  name                = "k6tests-amw${var.suffix}"
  resource_group_name = azurerm_resource_group.k6tests.name
  location            = azurerm_resource_group.k6tests.location
}

resource "azurerm_log_analytics_workspace" "k6tests" {
  name                = "k6tests-law${var.suffix}"
  location            = azurerm_resource_group.k6tests.location
  resource_group_name = azurerm_resource_group.k6tests.name
  daily_quota_gb      = var.log_analytics_workspace_daily_quota_gb
  retention_in_days   = var.log_analytics_workspace_retention_in_days
}

locals {
  streams = [
    "Microsoft-ContainerLog",
    "Microsoft-KubeEvents",
    "Microsoft-KubePodInventory",
    "Microsoft-KubeNodeInventory"
  ]

  # https://learn.microsoft.com/en-us/azure/azure-monitor/containers/container-insights-data-collection-configure?tabs=cli#configuration-file
  data_collection_settings = {
    "dataCollectionSettings" : {
      "interval" : "1m",
      "namespaceFilteringMode" : "Include",
      "namespaces" : concat(
        ["platform"],  # This can probably be removed once we "onboard ourselves"; else the code is here for other "system namespaces" we may care about
        var.namespaces # Team namespaces
      ),
      "enableContainerLogV2" : false
    }
  }
}

resource "azurerm_monitor_data_collection_rule" "k6tests" {
  name                = "k6tests-dcr${var.suffix}"
  resource_group_name = azurerm_resource_group.k6tests.name
  location            = azurerm_resource_group.k6tests.location

  destinations {
    log_analytics {
      workspace_resource_id = azurerm_log_analytics_workspace.k6tests.id
      name                  = "ciworkspace"
    }
  }

  data_flow {
    streams      = local.streams
    destinations = ["ciworkspace"]
  }

  data_sources {
    extension {
      streams        = local.streams
      extension_name = "ContainerInsights"
      extension_json = jsonencode(local.data_collection_settings)
      name           = "ContainerInsightsExtension"
    }
  }

  description = "DCR for Azure Monitor Container Insights"
}

resource "azurerm_monitor_data_collection_rule_association" "k6tests" {
  name                    = "ContainerInsightsExtension"
  target_resource_id      = azurerm_kubernetes_cluster.k6tests.id
  data_collection_rule_id = azurerm_monitor_data_collection_rule.k6tests.id
  description             = "Association of container insights data collection rule. Deleting this association will break the data collection for this AKS Cluster."
}
