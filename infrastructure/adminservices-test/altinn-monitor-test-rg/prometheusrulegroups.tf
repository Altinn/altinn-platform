// A resource group to host the PrometheusRuleGroups managed by the dis-promrulegroups-operator
resource "azurerm_resource_group" "promctl" {
  name     = "prom-rule-groups-rg"
  location = "norwayeast"
  tags = {
    "app" = "dis-promrulegroups-operator"
  }
}
