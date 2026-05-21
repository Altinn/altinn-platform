resource "azurerm_virtual_network" "gh_runners" {
  name                = "${var.repository_name}-runners"
  address_space       = [var.private_runners_address_space]
  location            = "norwayeast"
  resource_group_name = var.resource_group_name

  lifecycle {
    ignore_changes = [tags["costcenter"], tags["solution"]]
  }
}

resource "azurerm_subnet" "gh_runners" {
  name                 = "${var.repository_name}-runners"
  resource_group_name  = var.resource_group_name
  virtual_network_name = azurerm_virtual_network.gh_runners.name
  address_prefixes     = [var.private_runners_address_space]
  service_endpoints    = ["Microsoft.KeyVault"]

  delegation {
    name = "Microsoft.App.environments"
    service_delegation {
      name    = "Microsoft.App/environments"
      actions = ["Microsoft.Network/virtualNetworks/subnets/join/action"]
    }
  }
}

module "container_apps_gh_runners" {
  source      = "Altinn/altinn-modules/azurerm//modules/github_runner_container_app_jobs"
  version     = "1.2.2"
  app_id      = var.altinn_app_id
  install_id  = var.altinn_app_install_id
  app_key     = var.altinn_app_key
  kv_ip_rules = [var.host_ip]
  owner       = "Altinn"
  repos = [
    var.repository_name
  ]
  resource_prefix          = var.private_runners_prefix
  infrastructure_subnet_id = azurerm_subnet.gh_runners.id
  resource_group_name      = var.resource_group_name
  runner_cpu               = "2.0"
  runner_memory            = "4Gi"
  # renovate: datasource=docker depName=ghcr.io/altinn/altinn-platform/gh-runner
  runner_image = "ghcr.io/altinn/altinn-platform/gh-runner:v0.6.2"
  additional_tags = merge(
    var.tags,
    { submodule = "${var.repository_name}-github-runners" }
  )
}
