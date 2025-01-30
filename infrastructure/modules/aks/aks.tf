resource "azurerm_resource_group" "aks" {
  name     = "${var.prefix}-${var.environment}-aks-rg"
  location = var.location
}

resource "azurerm_kubernetes_cluster" "aks" {
  lifecycle {
    ignore_changes = [
      workload_autoscaler_profile,
      default_node_pool[0].node_count,
      windows_profile,
    ]
  }
  name                      = "${var.prefix}-${var.environment}-aks"
  location                  = azurerm_resource_group.aks.location
  resource_group_name       = azurerm_resource_group.aks.name
  dns_prefix                = "${var.prefix}-${var.environment}"
  sku_tier                  = var.aks_sku_tier
  kubernetes_version        = var.kubernetes_version
  automatic_upgrade_channel = "patch"
  node_os_upgrade_channel   = "NodeImage"
  oidc_issuer_enabled       = true
  workload_identity_enabled = true

  default_node_pool {
    name                         = "syspool"
    os_sku                       = "AzureLinux"
    vnet_subnet_id               = azurerm_subnet.aks["aks_syspool"].id
    only_critical_addons_enabled = true
    temporary_name_for_rotation  = "syspool99"
    max_pods                     = 200
    auto_scaling_enabled         = var.pool_configs["syspool"].auto_scaling_enabled
    node_count                   = var.pool_configs["syspool"].node_count
    vm_size                      = var.pool_configs["syspool"].vm_size
    min_count                    = var.pool_configs["syspool"].min_count
    max_count                    = var.pool_configs["syspool"].max_count
    zones                        = ["1", "2", "3"]
    orchestrator_version         = var.kubernetes_version

    upgrade_settings {
      max_surge = "10%"
    }
  }

  network_profile {
    network_plugin      = "azure"
    network_plugin_mode = "overlay"
    ip_versions         = ["IPv4", "IPv6"] # Azure did not like IPv6 first
    pod_cidrs           = ["10.240.0.0/16", "fd10:59f0:8c79:240::/64"]
    service_cidrs       = ["10.250.0.0/24", "fd10:59f0:8c79:250::/108"]
    dns_service_ip      = "10.250.0.10"
    load_balancer_sku   = "standard"

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

  # azure_active_directory_role_based_access_control {
  #   azure_rbac_enabled = true
  # }

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
    ]
  }
  name                  = "workpool"
  os_sku                = "AzureLinux"
  kubernetes_cluster_id = azurerm_kubernetes_cluster.aks.id
  vnet_subnet_id        = azurerm_subnet.aks["aks_workpool"].id
  max_pods              = 200
  auto_scaling_enabled  = var.pool_configs["workpool"].auto_scaling_enabled
  node_count            = var.pool_configs["workpool"].node_count
  vm_size               = var.pool_configs["workpool"].vm_size
  min_count             = var.pool_configs["workpool"].min_count
  max_count             = var.pool_configs["workpool"].max_count
  zones                 = ["1", "2", "3"]
  orchestrator_version  = var.kubernetes_version
  upgrade_settings {
    max_surge = "10%"
  }
}
