resource "azurerm_monitor_data_collection_rule" "law" {
  name                = "${azurerm_log_analytics_workspace.aks.name}-mdcr"
  resource_group_name = azurerm_resource_group.monitor.name
  location            = azurerm_resource_group.monitor.location

  destinations {
    log_analytics {
      workspace_resource_id = azurerm_log_analytics_workspace.aks.id
      name                  = azurerm_log_analytics_workspace.aks.name
    }
  }

  data_flow {
    streams = [
      "Microsoft-ContainerLog",
      "Microsoft-ContainerLogV2",
      "Microsoft-KubeEvents",
      "Microsoft-KubePodInventory"
    ]
    destinations = ["${azurerm_log_analytics_workspace.aks.name}"]
  }

  data_flow {
    streams      = ["Microsoft-Syslog"]
    destinations = ["${azurerm_log_analytics_workspace.aks.name}"]
  }

  data_sources {
    syslog {
      streams = ["Microsoft-Syslog"]
      facility_names = [
        "auth",
        "authpriv",
        "cron",
        "daemon",
        "mark",
        "kern",
        "local0",
        "local1",
        "local2",
        "local3",
        "local4",
        "local5",
        "local6",
        "local7",
        "lpr",
        "mail",
        "news",
        "syslog",
        "user",
        "uucp"
      ]
      log_levels = [
        "Error",
        "Critical",
        "Alert",
        "Emergency"
      ]
      name = "sysLogsDataSource"
    }

    extension {
      streams = [
        "Microsoft-ContainerLog",
        "Microsoft-ContainerLogV2",
        "Microsoft-KubeEvents",
        "Microsoft-KubePodInventory"
      ]
      extension_name = "ContainerInsights"
      extension_json = jsonencode({
        "dataCollectionSettings" : {
          "interval" : "5m",
          "namespaceFilteringMode" : "Exclude",
          "namespaces" : [
            "kube-system",
            "gatekeeper-system",
            "azure-arc"
          ],
          "enableContainerLogV2" : true

          log_collection_settings = {
            stdout = {
              enabled            = false
              exclude_namespaces = ["kube-system"]
            },
            stderr = {
              enabled            = true
              exclude_namespaces = ["kube-system", "monitoring", "linkerd-viz"]
            },
            env_var = {
              enabled = false
            },
            enrich_container_logs = {
              enabled = false
            },
            collect_all_kube_events = {
              enabled = false
            }
          }
        }
      })
      name = "ContainerInsightsExtension"
    }
  }

  description = "DCR for Azure Monitor Container Insights"
}

resource "azurerm_monitor_data_collection_rule_association" "law" {
  name                    = "${azurerm_log_analytics_workspace.aks.name}-mdcra"
  target_resource_id      = azurerm_kubernetes_cluster.aks.id
  data_collection_rule_id = azurerm_monitor_data_collection_rule.law.id
  description             = "Association of container insights data collection rule. Deleting this association will break the data collection for this AKS Cluster."
}
