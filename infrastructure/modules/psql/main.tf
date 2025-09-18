resource "azurerm_subnet" "psql" {
  count = var.psql_enable_vnet_integration ? 1 : 0
  name  = coalesce(var.psql_subnet_name, "${var.psql_ServerName}-subnet")
  resource_group_name               = var.psql_NetworkResourceGroup
  virtual_network_name              = var.psql_NetworkName
  private_endpoint_network_policies = "Enabled"
  address_prefixes                  = [var.psql_SubnetCidr]
  service_endpoints                 = ["Microsoft.Storage"]
  delegation {
    name = "fs"
    service_delegation {
      name = "Microsoft.DBforPostgreSQL/flexibleServers"
      actions = [
        "Microsoft.Network/virtualNetworks/subnets/join/action",
      ]
    }
  }
}

resource "azurerm_private_dns_zone" "psql" {
  lifecycle {
    ignore_changes = [
      tags["costcenter"],
      tags["solution"],
    ]
  }
  count               = var.psql_enable_vnet_integration ? 1 : 0
  name                = "${var.psql_ServerName}.privatelink.postgres.database.azure.com"
  resource_group_name = var.psql_ResourceGroup

  tags = {
    env     = "${var.environment}"
    product = "${var.product}"
    org     = "${var.organization}"
    managed = "terraform"
  }
}

resource "azurerm_private_dns_zone_virtual_network_link" "psql" {
  lifecycle {
    ignore_changes = [
      tags["costcenter"],
      tags["solution"],
    ]
  }
  count                 = var.psql_enable_vnet_integration ? 1 : 0
  name                  = "${var.psql_ServerName}-link"
  private_dns_zone_name = azurerm_private_dns_zone.psql[0].name
  virtual_network_id    = data.azurerm_virtual_network.psql.id
  resource_group_name   = var.psql_ResourceGroup
}

resource "azurerm_user_assigned_identity" "psql_identity" {
  lifecycle {
    ignore_changes = [
      tags["costcenter"],
      tags["solution"],
    ]
  }
  name                = "${var.psql_ServerName}-identity"
  resource_group_name = var.psql_ResourceGroup
  location            = var.location

  tags = {
    env     = "${var.environment}"
    product = "${var.product}"
    org     = "${var.organization}"
    managed = "terraform"
  }
}
resource "azurerm_postgresql_flexible_server" "psql" {
  lifecycle {
    ignore_changes = [
      tags["costcenter"],
      tags["solution"],
      zone,
      high_availability.0.standby_availability_zone,
      storage_mb,  # Always ignore to avoid drift after AutoGrow
    ]
  }
  name                            = "${var.psql_ServerName}"
  resource_group_name             = var.psql_ResourceGroup
  location                        = var.location
  version                         = var.psql_Version
  delegated_subnet_id             = var.psql_enable_vnet_integration ? azurerm_subnet.psql[0].id : null #azurerm_subnet.psql.id
  private_dns_zone_id             = var.psql_enable_vnet_integration ? azurerm_private_dns_zone.psql[0].id : null #azurerm_private_dns_zone.psql.id
  public_network_access_enabled   = var.psql_enable_vnet_integration ? false : true
  backup_retention_days           = var.psql_backup_retention_days
  geo_redundant_backup_enabled    = var.psql_GeoRedundantBackup

  storage_mb                      = var.psql_StorageSize
  auto_grow_enabled               = var.psql_StorageAutoGrow 

  sku_name                        = var.psql_ComputeSize
  storage_tier                    = var.psql_StorageTier == null ? null : var.psql_StorageTier
  depends_on                      = [azurerm_private_dns_zone_virtual_network_link.psql]

  authentication {
    active_directory_auth_enabled = true
    password_auth_enabled         = false
    tenant_id                     = data.azurerm_client_config.current.tenant_id
  }

  identity {
    type         = "UserAssigned"
    identity_ids = [azurerm_user_assigned_identity.psql_identity.id]
  }

  dynamic "high_availability" {
    for_each = var.psql_HighAvailability == true ? [1] : []
    content {
      mode = "ZoneRedundant"
    }
  }

  maintenance_window {
    day_of_week  = tostring(var.psql_maintenance_day_of_week)
    start_hour   = tostring(var.psql_maintenance_start_hour)
    start_minute = tostring(var.psql_maintenance_start_minute)
  }

  tags = {
    env     = "${var.environment}"
    product = "${var.product}"
    org     = "${var.organization}"
    managed = "terraform"
  }
}

resource "azurerm_postgresql_flexible_server_configuration" "pgbouncer_enabled" {
  count     = var.psql_pgbouncer ? 1 : 0
  name      = "pgbouncer.enabled"
  server_id = azurerm_postgresql_flexible_server.psql.id
  value     = "true"
}

resource "azurerm_postgresql_flexible_server_configuration" "pgbouncer_pool_mode" {
  count     = var.psql_pgbouncer ? 1 : 0
  name      = "pgbouncer.pool_mode"
  server_id = azurerm_postgresql_flexible_server.psql.id
  value     = var.psql_pgbouncer_pool_mode

  depends_on = [
    azurerm_postgresql_flexible_server_configuration.pgbouncer_enabled
  ]
}

resource "azurerm_postgresql_flexible_server_active_directory_administrator" "psql_terraform" {
  resource_group_name = azurerm_postgresql_flexible_server.psql.resource_group_name
  server_name         = azurerm_postgresql_flexible_server.psql.name
  tenant_id           = data.azurerm_client_config.current.tenant_id
  object_id           = "641fc568-3e2f-4174-a7ce-d91f50c8e6d6"
  principal_name      = "altinn-platform-terraform"
  principal_type      = "ServicePrincipal"
}
resource "azurerm_postgresql_flexible_server_active_directory_administrator" "psql_admin" {
  for_each            = data.azuread_group.psql_admin_groups
  server_name         = azurerm_postgresql_flexible_server.psql.name
  resource_group_name = azurerm_postgresql_flexible_server.psql.resource_group_name
  tenant_id           = data.azurerm_client_config.current.tenant_id
  object_id           = each.value.object_id
  principal_name      = each.value.display_name
  principal_type      = "Group"

  depends_on = [
    azurerm_postgresql_flexible_server_active_directory_administrator.psql_terraform
  ]
}



resource "azurerm_management_lock" "flexible_server" {
  count      = var.locks_off ? 0 : 1
  depends_on = [azurerm_postgresql_flexible_server.psql]
  name       = "resource-lock-flexible_server"
  scope      = azurerm_postgresql_flexible_server.psql.id
  lock_level = "CanNotDelete"
  notes      = "do not delete !!"
}

resource "azurerm_postgresql_flexible_server_database" "psql" {
  name      = var.psql_dbname
  server_id = azurerm_postgresql_flexible_server.psql.id
  collation = var.psql_DatabaseCollation
  charset   = "utf8"
}

resource "azurerm_postgresql_flexible_server_virtual_endpoint" "psql" {
  count             = var.psql_enable_endpoint ? 1 : 0
  name              = var.psql_endpoint_name
  source_server_id  = azurerm_postgresql_flexible_server.psql.id
  replica_server_id = azurerm_postgresql_flexible_server.psql.id
  type              = "ReadWrite"
}


locals {
  psql_extensions_value = (
    trimspace(var.psql_extensions) == "" ?
    null :
    join(",", distinct(compact([
      for e in split(",", lower(replace(var.psql_extensions, " ", ""))) : e
    ])))
  )
}


resource "azurerm_postgresql_flexible_server_configuration" "psql_extensions" {
  count     = local.psql_extensions_value == null ? 0 : 1
  name      = "azure.extensions"
  server_id = azurerm_postgresql_flexible_server.psql.id
  value     = local.psql_extensions_value
}

locals {

  psql_shared_preload_libraries_value = (
    trimspace(var.psql_shared_preload_libraries) == "" ?
    null :
    join(",", distinct(compact([
      for e in split(",", lower(replace(var.psql_shared_preload_libraries, " ", ""))) : e
    ])))
  )
}

resource "azurerm_postgresql_flexible_server_configuration" "psql_shared_preload_libraries" {
  count     = local.psql_shared_preload_libraries_value == null ? 0 : 1
  name      = "shared_preload_libraries"
  server_id = azurerm_postgresql_flexible_server.psql.id
  value     = local.psql_shared_preload_libraries_value
}

locals {
  reserved_pg_configs = [
    "azure.extensions",
    "shared_preload_libraries",
    "pgbouncer.enabled",
    "pgbouncer.pool_mode",
  ]

  effective_custom_pg_configs = {
    for k, v in var.psql_custom_configurations :
    k => v
    if !contains(local.reserved_pg_configs, k)
  }
}

resource "azurerm_postgresql_flexible_server_configuration" "custom" {
  for_each = local.effective_custom_pg_configs

  name      = each.key
  server_id = azurerm_postgresql_flexible_server.psql.id
  value     = each.value
}


data "azapi_resource" "psql_actual" {
  count  = var.psql_track_actual_storage ? 1 : 0
  type   = "Microsoft.DBforPostgreSQL/flexibleServers@2023-12-01-preview"
  name   = azurerm_postgresql_flexible_server.psql.name
  parent_id = "/subscriptions/${data.azurerm_client_config.current.subscription_id}/resourceGroups/${var.psql_ResourceGroup}"
  response_export_values = ["properties.storage.sizeGb"]
  depends_on = [azurerm_postgresql_flexible_server.psql]
}

locals {
  psql_actual_storage_mb = (
    var.psql_track_actual_storage && length(data.azapi_resource.psql_actual) > 0
    ? try(tonumber(data.azapi_resource.psql_actual[0].output.properties.storage.sizeGb) * 1024, azurerm_postgresql_flexible_server.psql.storage_mb)
    : azurerm_postgresql_flexible_server.psql.storage_mb
  )
}

resource "azurerm_postgresql_flexible_server_firewall_rule" "psql" {
  for_each = var.psql_enable_vnet_integration ? {} : var.psql_firewall_rules
  name      = each.key
  server_id = azurerm_postgresql_flexible_server.psql.id
  start_ip_address = each.value.start_ip
  end_ip_address   = each.value.end_ip
}

#Example of peering
/*
resource "azurerm_virtual_network_peering" "psql_to_aks" {
  name                      = var.aks_vnet_name
  resource_group_name       = azurerm_resource_group.psql.name
  virtual_network_name      = azurerm_virtual_network.psql.name
  remote_virtual_network_id = var.aks_vnet_id

  allow_virtual_network_access = true
  allow_forwarded_traffic      = true
  allow_gateway_transit        = false
  use_remote_gateways          = false
}
resource "azurerm_virtual_network_peering" "aks_to_psql" {
  name                      = azurerm_virtual_network.psql.name
  resource_group_name       = var.aks_resource_group_name
  virtual_network_name      = var.aks_vnet_name
  remote_virtual_network_id = azurerm_virtual_network.psql.id

  allow_virtual_network_access = true
  allow_forwarded_traffic      = true
  allow_gateway_transit        = false
  use_remote_gateways          = false
}
*/