data "azurerm_client_config" "current" {}

# Dialogporten
data "azurerm_resource_group" "rg_dp_test" {
  name     = "dp-be-test-rg"
  location = "norwayeast"
}

data "azurerm_log_analytics_workspace" "dp_law_test" {
  name                = "dp-be-test-insightsWorkspace"
  resource_group_name = data.azurerm_resource_group.rg_dp_test.name
}
