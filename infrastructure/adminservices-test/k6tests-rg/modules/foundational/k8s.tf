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
    name                        = "default"
    auto_scaling_enabled        = true
    min_count                   = 1
    max_count                   = 3
    vm_size                     = "Standard_D3_v2"
    temporary_name_for_rotation = "tmpdefault"
    max_pods                    = 200

    upgrade_settings { # Adding these to keep plans clean
      drain_timeout_in_minutes      = 0
      max_surge                     = "10%"
      node_soak_duration_in_minutes = 0
    }
  }

  network_profile {
    network_plugin = "kubenet"
    load_balancer_profile {
      outbound_ports_allocated = 5000 # https://learn.microsoft.com/en-us/azure/aks/configure-load-balancer-standard?tabs=create-cluster-ip-based%2Cupdate-cluster-managed-outbound-ips%2Ccreate-cluster-custom-ips%2Ccreate-cluster-custom-ip-prefixes%2Ccreate-cluster-outbound-ports-ips%2Ccreate-cluster-idle-timeout#configure-the-allocated-outbound-ports
    }
  }
}

resource "azurerm_kubernetes_cluster_node_pool" "spot" {
  name                  = "spot"
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k6tests.id
  vm_size               = "Standard_D3_v2"
  auto_scaling_enabled  = true
  node_count            = 0
  min_count             = 0
  max_count             = 3
  priority              = "Spot"
  eviction_policy       = "Delete"
  spot_max_price        = -1 # (the current on-demand price for a Virtual Machine)
  max_pods              = 200
  node_labels = {
    "kubernetes.azure.com/scalesetpriority" : "spot", # Automatically added by Azure
    spot : true
  }
  node_taints = [
    "kubernetes.azure.com/scalesetpriority=spot:NoSchedule", # Automatically added by Azure
  ]

  lifecycle {
    ignore_changes = [
      node_count
    ]
  }

  temporary_name_for_rotation = "tmpspot"
}

resource "azurerm_kubernetes_cluster_node_pool" "spot8c28g" {
  name                  = "spot8c28g"
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k6tests.id
  vm_size               = "Standard_D4_v2"
  auto_scaling_enabled  = true
  node_count            = 0
  min_count             = 0
  max_count             = 3
  priority              = "Spot"
  eviction_policy       = "Delete"
  spot_max_price        = -1 # (the current on-demand price for a Virtual Machine)
  node_labels = {
    "kubernetes.azure.com/scalesetpriority" : "spot", # Automatically added by Azure
    spot8cpu28gbmem : true
  }
  node_taints = [
    "kubernetes.azure.com/scalesetpriority=spot:NoSchedule", # Automatically added by Azure
  ]
  temporary_name_for_rotation = "tmpd8c28g"
}


resource "azurerm_kubernetes_cluster_node_pool" "spot1c3g" {
  name                  = "spot1c3g"
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k6tests.id
  vm_size               = "Standard_D1_v2"
  auto_scaling_enabled  = true
  node_count            = 0
  min_count             = 0
  max_count             = 40
  priority              = "Spot"
  eviction_policy       = "Delete"
  spot_max_price        = -1 # (the current on-demand price for a Virtual Machine)
  node_labels = {
    "kubernetes.azure.com/scalesetpriority" : "spot", # Automatically added by Azure
    spot1cpu3gbmem : true
  }
  node_taints = [
    "kubernetes.azure.com/scalesetpriority=spot:NoSchedule", # Automatically added by Azure
  ]
  temporary_name_for_rotation = "tmpd1c3g"
}

resource "azurerm_kubernetes_cluster_node_pool" "spot2c7g" {
  name                  = "spot2c7g"
  kubernetes_cluster_id = azurerm_kubernetes_cluster.k6tests.id
  vm_size               = "Standard_D2_v2"
  auto_scaling_enabled  = true
  node_count            = 0
  min_count             = 0
  max_count             = 20
  priority              = "Spot"
  eviction_policy       = "Delete"
  spot_max_price        = -1 # (the current on-demand price for a Virtual Machine)
  node_labels = {
    "kubernetes.azure.com/scalesetpriority" : "spot", # Automatically added by Azure
    spot2cpu7gbmem : true
  }
  node_taints = [
    "kubernetes.azure.com/scalesetpriority=spot:NoSchedule", # Automatically added by Azure
  ]
  temporary_name_for_rotation = "tmpd2c7g"
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
  node_taints = [
    "workload=prometheus:NoSchedule"
  ]

  upgrade_settings {
    drain_timeout_in_minutes      = 0
    max_surge                     = "10%"
    node_soak_duration_in_minutes = 0
  }
  temporary_name_for_rotation = "tmpprom"
}
