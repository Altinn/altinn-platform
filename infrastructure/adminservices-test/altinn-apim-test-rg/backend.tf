# Deploy dis-demo-pgsl https://github.com/dis-hackaton/demo-apps/tree/main/dis-demo-pgsql in azure conatiner app to serve as backend

resource "random_string" "name" {
  length  = 6
  special = false
  upper   = false
  numeric = false
}

resource "azurerm_user_assigned_identity" "acaghr_managed_identity" {
  name                = "${var.name_prefix}-${random_string.name.result}-aca-mi"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
}

resource "azurerm_container_app_environment" "container_app_environment" {
  name                = "${var.name_prefix}-${random_string.name.result}-acaenv"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
}

resource "azurerm_container_app" "container_app" {
  name                         = "${var.name_prefix}-${random_string.name.result}-aca"
  resource_group_name          = azurerm_resource_group.rg.name
  container_app_environment_id = azurerm_container_app_environment.container_app_environment.id
  revision_mode                = "Single"
  identity {
    type = "UserAssigned"
    identity_ids = [
      azurerm_user_assigned_identity.acaghr_managed_identity.id
    ]
  }
  registry {
    identity = azurerm_user_assigned_identity.acaghr_managed_identity.id
    server   = data.azurerm_container_registry.altinncr.login_server
  }
  template {
    container {
      name   = "dis-demo-pgsql"
      image  = "${data.azurerm_container_registry.altinncr.login_server}/dis-hackaton/dis-demo-pgsql:latest"
      cpu    = "0.5"
      memory = "1Gi"
      args = [
        "webserver",
        "--auth-enabled"
      ]
    }
    min_replicas = 0
    max_replicas = 1
    http_scale_rule {
      name                = "http-scale-rule"
      concurrent_requests = 1000
    }
  }
}

resource "azurerm_role_assignment" "altinncr_acrpull" {
  principal_id                     = azurerm_user_assigned_identity.acaghr_managed_identity.principal_id
  role_definition_name             = "AcrPull"
  scope                            = data.azurerm_container_registry.altinncr.id
  skip_service_principal_aad_check = true
}