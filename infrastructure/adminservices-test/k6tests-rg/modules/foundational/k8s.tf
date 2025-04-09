resource "azurerm_kubernetes_cluster" "k6tests" {
  name                = "k6tests-cluster${var.suffix}"
  location            = azurerm_resource_group.k6tests.location
  resource_group_name = azurerm_resource_group.k6tests.name
  dns_prefix          = "k6tests-cluster${var.suffix}"

  workload_identity_enabled = true
  oidc_issuer_enabled       = true
  identity {
    type = "SystemAssigned"
  }

  local_account_disabled            = true
  role_based_access_control_enabled = true
  azure_active_directory_role_based_access_control {
    tenant_id              = var.tenant_id
    admin_group_object_ids = var.k8s_admin_group_object_ids
    azure_rbac_enabled     = false
  }

  oms_agent {
    log_analytics_workspace_id      = azurerm_log_analytics_workspace.k6tests.id
    msi_auth_for_monitoring_enabled = true
  }

  automatic_upgrade_channel = "stable"

  default_node_pool {
    name                 = "default"
    auto_scaling_enabled = true
    min_count            = 1
    max_count            = 3
    vm_size              = "Standard_D3_v2"
    temporary_name_for_rotation = "tmpdefault"
    max_pods = 200

    upgrade_settings { # Adding these to keep plans clean
      drain_timeout_in_minutes      = 0
      max_surge                     = "10%"
      node_soak_duration_in_minutes = 0
    }
  }
}

resource "azurerm_kubernetes_cluster_node_pool" "spot" {
  name                  = "spot"
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k6tests.id
  vm_size               = "Standard_DS2_v2"
  auto_scaling_enabled  = true
  node_count            = 0
  min_count             = 0
  max_count             = 10
  priority              = "Spot"
  eviction_policy       = "Delete"
  spot_max_price        = -1 # (the current on-demand price for a Virtual Machine)
  max_pods              = 200
  node_labels = {
    "kubernetes.azure.com/scalesetpriority" : "spot", # Automatically added by Azure
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
  node_labels = {
    workload : "prometheus"
  }
  node_taints = ["workload=prometheus:NoSchedule"]
}
