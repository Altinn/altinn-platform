resource "azurerm_log_analytics_workspace" "k6tests" {
  name                = "k6tests-law"
  location            = azurerm_resource_group.k6tests_rg.location
  resource_group_name = azurerm_resource_group.k6tests_rg.name
  daily_quota_gb      = 5 # TODO: check how many logs we are generating and tweak accordingly
  retention_in_days   = 30
}

resource "azurerm_monitor_data_collection_rule" "k6tests" {
  name                = "k6tests-dcr"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.k6tests_rg.location

  destinations {
    log_analytics {
      workspace_resource_id = azurerm_log_analytics_workspace.k6tests.id
      name                  = "ciworkspace"
    }
  }

  data_flow {
    streams      = ["Microsoft-ContainerLog", "Microsoft-KubeEvents", "Microsoft-KubePodInventory", "Microsoft-KubeNodeInventory"]
    destinations = ["ciworkspace"]
  }

  data_sources {
    extension {
      streams        = ["Microsoft-ContainerLog", "Microsoft-KubeEvents", "Microsoft-KubePodInventory", "Microsoft-KubeNodeInventory"]
      extension_name = "ContainerInsights"
      extension_json = jsonencode({
        "dataCollectionSettings" : {
          "interval" : "1m",
          "namespaceFilteringMode" : "Include",
          "namespaces" : ["dialogporten", "correspondence"] # Only in the namespaces we have k6 tests running for now.
          "enableContainerLogV2" : false
        }
      })
      name = "ContainerInsightsExtension"
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

resource "azurerm_kubernetes_cluster" "k6tests" {
  name                = "k6tests-cluster"
  location            = azurerm_resource_group.k6tests_rg.location
  resource_group_name = azurerm_resource_group.k6tests_rg.name
  dns_prefix          = "k6tests-cluster"

  default_node_pool {
    name                 = "default"
    auto_scaling_enabled = true
    min_count            = 1
    max_count            = 10
    vm_size              = "Standard_D3_v2"
    upgrade_settings { # Adding these to keep plans clean
      drain_timeout_in_minutes      = 0
      max_surge                     = "10%"
      node_soak_duration_in_minutes = 0
    }
    temporary_name_for_rotation = "tmpdefault"
  }

  workload_identity_enabled = true
  oidc_issuer_enabled       = true

  identity {
    type = "SystemAssigned"
  }

  local_account_disabled            = true
  role_based_access_control_enabled = true
  azure_active_directory_role_based_access_control {
    # tenant_id = "" # Optional
    admin_group_object_ids = ["c9c317cc-aec0-4c8b-bdad-b54333686e8a"]
    azure_rbac_enabled     = false
  }

  oms_agent {
    log_analytics_workspace_id      = azurerm_log_analytics_workspace.k6tests.id
    msi_auth_for_monitoring_enabled = true
  }

}

resource "azurerm_kubernetes_cluster_node_pool" "spot" {
  name                  = "spot"
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k6tests.id
  vm_size               = "Standard_DS2_v2"
  auto_scaling_enabled  = true
  node_count            = 0
  min_count             = 0
  max_count             = 1
  priority              = "Spot"
  eviction_policy       = "Delete"
  spot_max_price        = -1 # (the current on-demand price for a Virtual Machine)
  node_labels = {
    "kubernetes.azure.com/scalesetpriority" : "spot", # Automatically added by Azure
    spot : true
  }
  node_taints = [
    "kubernetes.azure.com/scalesetpriority=spot:NoSchedule", # Automatically added by Azure
  ]
}

resource "azurerm_kubernetes_cluster_node_pool" "prometheus" {
  name                  = "prometheus"
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k6tests.id
  vm_size               = "Standard_D3_v2"
  auto_scaling_enabled  = false
  node_count            = 1
  priority              = "Spot" # Spot since we are still testing
  eviction_policy       = "Delete"
  spot_max_price        = -1 # (the current on-demand price for a Virtual Machine)
  node_labels = {
    "kubernetes.azure.com/scalesetpriority" : "spot", # Automatically added by Azure
    prometheus : true
  }
  node_taints = [
    "kubernetes.azure.com/scalesetpriority=spot:NoSchedule", # Automatically added by Azure
    "workload=prometheus:NoSchedule",
  ]
}
