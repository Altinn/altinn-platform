resource "azurerm_kubernetes_cluster" "k6tests" {
  name                = "k6tests-cluster"
  location            = azurerm_resource_group.k6tests_rg.location
  resource_group_name = azurerm_resource_group.k6tests_rg.name
  dns_prefix          = "k6tests-cluster"

  default_node_pool {
    name       = "default"
    node_count = 1
    vm_size    = "Standard_D2_v2"
  }

  workload_identity_enabled = true
  oidc_issuer_enabled       = true

  identity {
    type = "SystemAssigned"
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
    spot : true
  }
}
