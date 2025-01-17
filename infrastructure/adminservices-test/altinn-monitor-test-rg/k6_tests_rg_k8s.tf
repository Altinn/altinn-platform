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
