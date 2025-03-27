resource "azurerm_kubernetes_cluster" "aks" {
  lifecycle {
    ignore_changes = [
      workload_autoscaler_profile,
      default_node_pool[0].node_count,
      windows_profile,
      kubernetes_version,
      default_node_pool[0].orchestrator_version,
    ]
  }
  name                      = "${var.name_prefix}-aks"
  location                  = azurerm_resource_group.rg.location
  resource_group_name       = azurerm_resource_group.rg.name
  dns_prefix                = var.name_prefix
  sku_tier                  = var.aks_sku_tier
  kubernetes_version        = var.kubernetes_version
  automatic_upgrade_channel = "patch"
  node_os_upgrade_channel   = "NodeImage"
  oidc_issuer_enabled       = true
  workload_identity_enabled = true

  default_node_pool {
    name                         = "syspool"
    os_sku                       = "AzureLinux"
    orchestrator_version         = var.kubernetes_version
    vnet_subnet_id               = azurerm_subnet.subnets["aks_syspool"].id
    only_critical_addons_enabled = true
    temporary_name_for_rotation  = "syspool99"
    auto_scaling_enabled         = true
    max_pods                     = 200
    vm_size                      = var.pool_configs["syspool"].vm_size
    min_count                    = var.pool_configs["syspool"].min_count
    max_count                    = var.pool_configs["syspool"].max_count
    zones                        = ["1", "2", "3"]
    upgrade_settings {
      max_surge = "10%"
    }
  }

  network_profile {
    network_plugin      = "azure"
    network_plugin_mode = "overlay"
    ip_versions         = ["IPv4", "IPv6"] # Azure did not like IPv6 first
    pod_cidrs           = ["10.240.0.0/16", "fd10:59f0:8c79:240::/64"]
    service_cidrs       = ["10.250.0.0/16", "fd10:59f0:8c79:250::/108"]
    dns_service_ip      = "10.250.0.53"
    load_balancer_profile {
      outbound_ip_prefix_ids = [
        azurerm_public_ip_prefix.prefix4.id,
        azurerm_public_ip_prefix.prefix6.id
      ]
    }
  }

  identity {
    type = "SystemAssigned"
  }

  monitor_metrics {}

  maintenance_window_auto_upgrade {
    frequency   = "Weekly"
    interval    = "1"
    duration    = "5"
    day_of_week = "Monday"
    start_time  = "23:30"
    utc_offset  = "+00:00"
  }
  maintenance_window_node_os {
    frequency   = "Weekly"
    interval    = "1"
    duration    = "5"
    day_of_week = "Tuesday"
    start_time  = "23:30"
    utc_offset  = "+00:00"
  }
}

resource "azurerm_kubernetes_cluster_node_pool" "workpool" {
  lifecycle {
    ignore_changes = [
      node_count,
      orchestrator_version,
    ]
  }
  name                  = "workpool"
  os_sku                = "AzureLinux"
  kubernetes_cluster_id = azurerm_kubernetes_cluster.aks.id
  vnet_subnet_id        = azurerm_subnet.subnets["aks_workpool"].id
  orchestrator_version  = var.kubernetes_version
  auto_scaling_enabled  = true
  max_pods              = 200
  vm_size               = var.pool_configs["workpool"].vm_size
  min_count             = var.pool_configs["workpool"].min_count
  max_count             = var.pool_configs["workpool"].max_count
  zones                 = ["1", "2", "3"]
  upgrade_settings {
    max_surge = "10%"
  }
}

resource "azurerm_role_assignment" "aks_id_rg_contributor" {
  scope                            = azurerm_resource_group.rg.id
  role_definition_name             = "Contributor"
  principal_id                     = azurerm_kubernetes_cluster.aks.identity[0].principal_id
  skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "altinncr_acrpull" {
  principal_id                     = azurerm_kubernetes_cluster.aks.kubelet_identity[0].object_id
  role_definition_name             = "AcrPull"
  scope                            = data.azurerm_container_registry.altinncr.id
  skip_service_principal_aad_check = true
}
