
locals {
  write_operations = <<-EOT
  (
    !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/write'})
    AND
    !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/add/action'})
    AND
    !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/runAsSuperUser/action'})
    AND
    !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/tags/write'})
    AND
    !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/delete'})
    AND
    !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/deleteBlobVersion/action'})
    AND
    !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/immutableStorage/runAsSuperUser/action'})
    AND
    !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/move/action'})
    AND
    !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/manageOwnership/action'})
    AND
    !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/permanentDelete/action'})
    AND
    !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/modifyPermissions/action'})
  )
  EOT
}

# https:#learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#storage
data "azurerm_role_definition" "storage_blob_data_owner" {
  role_definition_id = "b7e6dc6d-f1e8-4753-8033-0f276bb0955b"
}

# https:#learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#storage
data "azurerm_role_definition" "storage_blob_reader_data_access" {
  role_definition_id = "c12c1c16-33a1-487b-954d-41c89c60f349"
}

# https:#learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#general
data "azurerm_role_definition" "reader" {
  role_definition_id = "acdd72a7-3385-48ef-bd42-f606fba81ae7"
}

# https:#learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#general
data "azurerm_role_definition" "contributor" {
  role_definition_id = "b24988ac-6180-42a0-ab88-20f7382dd24c"
}

# https:#learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#general
data "azurerm_role_definition" "user_access_administrator" {
  role_definition_id = "18d7d88d-d35e-4fb5-a5c3-7773c20a72d9"
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/data-sources/resources
data "azurerm_resource_group" "tfstate" {
  name = var.arm_resource_group_name
}

# https:#registry.terraform.io/providers/hashicorp/azurerm/latest/docs/data-sources/billing_enrollment_account_scope
data "azurerm_billing_enrollment_account_scope" "billing" {
  billing_account_name    = var.arm_billing_account_name
  enrollment_account_name = var.arm_enrollment_account_scope

  count = var.arm_billing_account_name != null && var.arm_enrollment_account_scope != null ? 1 : 0
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/subscription
resource "azurerm_subscription" "subscriptions" {
  subscription_name = "${each.value.product_name}-${each.value.workspace_name}"
  billing_scope_id  = data.azurerm_billing_enrollment_account_scope.billing[0].id

  for_each = { for key, value in local.products : key => value if var.arm_billing_account_name != null && var.arm_enrollment_account_scope != null }
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/app_configuration
resource "azurerm_app_configuration" "state" {
  name                = "${var.arm_product_name}${var.arm_solution_name}appconf${var.arm_instance}"
  resource_group_name = data.azurerm_resource_group.tfstate.name
  location            = data.azurerm_resource_group.tfstate.location
  sku                 = "standard"

  tags = merge({

  }, local.default_tags)

  lifecycle {
    ignore_changes = [
      tags["costcenter"],
      tags["solution"],
    ]
  }
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_definition
resource "azurerm_role_definition" "app_config_list_keys_action" {
  name        = "app-configuration-list-keys-action"
  scope       = data.azurerm_resource_group.tfstate.id
  description = "Grants listKeys/action on App Configurations. Managed by terraform"

  permissions {
    actions     = ["Microsoft.AppConfiguration/configurationStores/listKeys/action"]
    not_actions = []
  }

  assignable_scopes = [
    azurerm_app_configuration.state.id
  ]
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/storage_account
resource "azurerm_storage_account" "backend" {
  name                     = "${var.arm_product_name}${var.arm_solution_name}storage${var.arm_instance}"
  resource_group_name      = data.azurerm_resource_group.tfstate.name
  location                 = data.azurerm_resource_group.tfstate.location
  account_kind             = "BlobStorage"
  access_tier              = "Hot"
  account_tier             = "Standard"
  account_replication_type = "GRS"

  blob_properties {
    versioning_enabled = true
  }

  tags = merge({

  }, local.default_tags)

  lifecycle {
    ignore_changes = [
      tags["costcenter"],
      tags["solution"],
    ]
  }
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/storage_container
resource "azurerm_storage_container" "container" {
  name               = "tfstates"
  storage_account_id = azurerm_storage_account.backend.id
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "product_admins_storage_blob_owner" {
  scope                = azurerm_storage_container.container.id
  principal_id         = azuread_group.product_admins.object_id
  role_definition_name = data.azurerm_role_definition.storage_blob_data_owner.name
  #  skip_service_principal_aad_check = true
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "product_admins_user_access_administrator" {
  scope                = azurerm_storage_container.container.id
  principal_id         = azuread_group.product_admins.object_id
  role_definition_name = data.azurerm_role_definition.user_access_administrator.name
  #  skip_service_principal_aad_check = true
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "product_admins_contributor" {
  scope                = data.azurerm_resource_group.tfstate.id
  principal_id         = azuread_group.product_admins.object_id
  role_definition_name = data.azurerm_role_definition.contributor.name
  #  skip_service_principal_aad_check = true
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "product_reader_storage_blob_reader_data_access" {
  scope                = azurerm_storage_account.backend.id
  principal_id         = azuread_group.product_readers.object_id
  role_definition_name = data.azurerm_role_definition.storage_blob_reader_data_access.name
  #  skip_service_principal_aad_check = true
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "product_reader_app_config_list_keys_action" {
  scope                = azurerm_app_configuration.state.id
  principal_id         = azuread_group.product_readers.object_id
  role_definition_name = azurerm_role_definition.app_config_list_keys_action.name
  #  skip_service_principal_aad_check = true
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "product_readers_storage_blob_owner" {
  scope                = azurerm_storage_container.container.id
  principal_id         = azuread_group.product_readers.object_id
  role_definition_name = data.azurerm_role_definition.storage_blob_data_owner.name
  condition_version    = "2.0"
  condition            = <<-EOT
  (
   ${local.write_operations}
   OR
   (
    @Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path] StringStartsWith 'github.com/${lower(local.configuration.admin.github.owner)}/${lower(local.configuration.admin.github.repository)}/'
   )
  )
  EOT
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "product_readers_reader" {
  scope                = data.azurerm_resource_group.tfstate.id
  principal_id         = azuread_group.product_readers.object_id
  role_definition_name = data.azurerm_role_definition.reader.name
  #  skip_service_principal_aad_check = true
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "products" {
  scope                = azurerm_storage_container.container.id
  principal_id         = azuread_group.admins[each.value.slug].object_id
  role_definition_name = data.azurerm_role_definition.storage_blob_data_owner.name

  depends_on = [azurerm_role_assignment.product_admins_user_access_administrator]

  condition_version = "2.0"
  condition         = <<-EOT
  (
    ${local.write_operations}
    OR
    (
      %{for repository in each.value.repositories.names}
        @Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path] StringStartsWith 'github.com/${lower(each.value.repositories.owner)}/${lower(repository)}'
        OR
      %{endfor~}
      @Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path] StringStartsWith 'github.com/${lower(each.value.repositories.owner)}/~/EOT'
    )
  )
  EOT

  for_each = local.role_abac_products
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "appregg" {
  scope                            = azurerm_storage_container.container.id
  principal_id                     = azuread_service_principal.product[each.key].object_id
  role_definition_name             = data.azurerm_role_definition.storage_blob_data_owner.name
  skip_service_principal_aad_check = true

  depends_on = [azurerm_role_assignment.product_admins_user_access_administrator]

  condition_version = "2.0"
  condition         = <<-EOT
  (
   ${local.write_operations}
   OR
   (
    %{for scope in each.value.scopes}
    @Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path] StringStartsWith 'github.com/${lower(each.value.repository.owner)}/${lower(each.value.repository.name)}/environments/${lower(scope.environment.name)}'
    OR
    %{endfor~}
    @Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path] StringStartsWith 'github.com/${lower(each.value.repository.owner)}/~/EOT'
   )
  )
  EOT

  for_each = local.role_abac_apps
}
