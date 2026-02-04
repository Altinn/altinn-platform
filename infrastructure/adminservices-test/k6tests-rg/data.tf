/*
data "azurerm_kubernetes_cluster" "k6tests" {
  depends_on          = [module.foundational]
  name                = module.foundational.k6tests_cluster_name
  resource_group_name = module.foundational.k6tests_resource_group_name
}

# Meh, not sure I like this one.
# It should be predicatable but there should be a better way.
data "azurerm_monitor_data_collection_endpoint" "k6tests" {
  depends_on          = [module.foundational]
  name                = "k6tests-amw${local.suffix}"
  resource_group_name = "MA_k6tests-amw${local.suffix}_norwayeast_managed"
}

data "azurerm_monitor_data_collection_rule" "k6tests" {
  depends_on          = [module.foundational]
  name                = "k6tests-dcr${local.suffix}"
  resource_group_name = module.foundational.k6tests_resource_group_name
}
*/
data "azurerm_client_config" "current" {}
