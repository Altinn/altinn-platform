data "azurerm_client_config" "current" {}

# Dialogporten
data "azurerm_resource_group" "rg_dp_stag" {
  name     = "dp-be-staging-rg"
  provider = azurerm.dp_stag
}

data "azurerm_resource_group" "rg_dp_prod" {
  name     = "dp-be-prod-rg"
  provider = azurerm.dp_prod
}

data "azurerm_resource_group" "rd_dp_test" {
  for_each = toset(values(var.insights_workspace_test_dp))
  name     = each.value
  provider = azurerm.dp_test
}

data "azurerm_log_analytics_workspace" "dp_law_test" {
  for_each = var.insights_workspace_test_dp

  name                = each.key
  resource_group_name = each.value
  provider            = azurerm.dp_test
}

data "azurerm_log_analytics_workspace" "dp_law_stag" {
  name                = "dp-be-staging-insightsWorkspace"
  resource_group_name = data.azurerm_resource_group.rg_dp_stag.name
  provider            = azurerm.dp_stag
}

data "azurerm_log_analytics_workspace" "dp_law_prod" {
  name                = "dp-be-prod-insightsWorkspace"
  resource_group_name = data.azurerm_resource_group.rg_dp_prod.name
  provider            = azurerm.dp_prod
}

# Studio
data "azurerm_resource_group" "rg_studio_test" {
  name     = "altinn-rg"
  provider = azurerm.studio_test
}

data "azurerm_resource_group" "rg_studio_prod" {
  name     = "altinn-rg"
  provider = azurerm.studio_prod
}

data "azurerm_log_analytics_workspace" "studio_law_test" {
  name                = "altinn-studio-test-log"
  resource_group_name = data.azurerm_resource_group.rg_studio_dev.name
  provider            = azurerm.studio_test
}

data "azurerm_log_analytics_workspace" "studio_law_prod" {
  name                = "altinn-studio-prod-log"
  resource_group_name = data.azurerm_resource_group.rg_studio_prod.name
  provider            = azurerm.studio_prod
}
