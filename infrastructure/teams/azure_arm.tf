
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

// https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#storage
data "azurerm_role_definition" "storage_blob_data_owner" {
  role_definition_id = "b7e6dc6d-f1e8-4753-8033-0f276bb0955b"
}

// https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#general
data "azurerm_role_definition" "reader" {
  role_definition_id = "acdd72a7-3385-48ef-bd42-f606fba81ae7"
}

// https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#general
data "azurerm_role_definition" "contributor" {
  role_definition_id = "b24988ac-6180-42a0-ab88-20f7382dd24c"
}

// https://learn.microsoft.com/en-us/azure/role-based-access-control/built-in-roles#general
data "azurerm_role_definition" "user_access_administrator" {
  role_definition_id = "18d7d88d-d35e-4fb5-a5c3-7773c20a72d9"
}


data "azurerm_resource_group" "tfstate" {
  name = "terraform-rg"
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/data-sources/billing_enrollment_account_scope
data "azurerm_billing_enrollment_account_scope" "billing" {
  billing_account_name    = var.arm_billing_account_name
  enrollment_account_name = var.arm_enrollment_account_scope

  count = var.arm_billing_account_name != null && var.arm_enrollment_account_scope != null ? 1 : 0
}

resource "azurerm_subscription" "subscriptions" {
  subscription_name = "${each.value.team.name}-${each.value.environment}"
  billing_scope_id  = data.azurerm_billing_enrollment_account_scope.billing[0].id

  for_each = { for key, value in local.teams : key => value if var.arm_billing_account_name != null && var.arm_enrollment_account_scope != null }
}

resource "azurerm_app_configuration" "state" {
  lifecycle {
    ignore_changes = [
      tags["costcenter"],
      tags["solution"]
    ]
  }
  name                = "${var.arm_product_name}${var.arm_solution_name}appconf${var.arm_instance}"
  resource_group_name = data.azurerm_resource_group.tfstate.name
  location            = data.azurerm_resource_group.tfstate.location
  sku                 = "standard"
}

resource "azurerm_storage_account" "backend" {
  lifecycle {
    ignore_changes = [
      tags["costcenter"],
      tags["solution"]
    ]
  }
  name                     = "${var.arm_product_name}${var.arm_solution_name}storage${var.arm_instance}"
  resource_group_name      = data.azurerm_resource_group.tfstate.name
  location                 = data.azurerm_resource_group.tfstate.location
  account_kind             = "BlobStorage"
  access_tier              = "Hot"
  account_tier             = "Standard"
  account_replication_type = "GRS"
}

resource "azurerm_storage_container" "container" {
  name                 = "tfstates"
  storage_account_name = azurerm_storage_account.backend.name
}

resource "azurerm_management_group" "parent" {
  name         = "Altinn-Teams"
  display_name = "Altinn-Teams"
}

resource "azurerm_management_group" "management_groups" {
  name                       = "${each.value.team.slug}-${title(each.value.environment.name)}"
  display_name               = "${replace(each.value.team.name, " ", "-")}-${title(each.value.environment.name)}"
  parent_management_group_id = azurerm_management_group.parent.id

  for_each = local.teams
}

resource "azurerm_management_group_subscription_association" "subscriptions" {
  management_group_id = azurerm_management_group.management_groups[each.key].id
  subscription_id     = azurerm_subscription.subscriptions[each.key].id

  for_each = { for key, value in local.teams : key => value if var.arm_billing_account_name != null && var.arm_enrollment_account_scope != null }
}

resource "azurerm_role_assignment" "administrator_user_access_administrator" {
  scope                = azurerm_management_group.parent.id
  principal_id         = azuread_service_principal.administrator.object_id
  role_definition_name = data.azurerm_role_definition.user_access_administrator.name
}

resource "azurerm_role_assignment" "administrator_contributor" {
  scope                = azurerm_management_group.parent.id
  principal_id         = azuread_service_principal.administrator.object_id
  role_definition_name = data.azurerm_role_definition.contributor.name
}

resource "azurerm_role_assignment" "apps_user_access_administrator" {
  scope                = azurerm_management_group.management_groups[each.value.team_slug].id
  principal_id         = azuread_service_principal.team[each.key].object_id
  role_definition_name = data.azurerm_role_definition.user_access_administrator.name

  for_each = local.app_reggs
}

resource "azurerm_role_assignment" "apps_contributor" {
  scope                = azurerm_management_group.management_groups[each.value.team_slug].id
  principal_id         = azuread_service_principal.team[each.key].object_id
  role_definition_name = data.azurerm_role_definition.contributor.name

  for_each = local.app_reggs
}

resource "azurerm_role_assignment" "client_storage_account_admin" {
  scope                = azurerm_storage_container.container.resource_manager_id
  principal_id         = azuread_group.terraform_admins.object_id
  role_definition_name = data.azurerm_role_definition.storage_blob_data_owner.name
  # skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "readers" {
  scope                = azurerm_management_group.management_groups[each.key].id
  principal_id         = azuread_group.readers[each.key].object_id
  role_definition_name = data.azurerm_role_definition.reader.name

  for_each = local.teams
  # skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "developers" {
  scope                = azurerm_management_group.management_groups[each.key].id
  principal_id         = azuread_group.developers[each.key].object_id
  role_definition_name = data.azurerm_role_definition.contributor.name

  for_each = local.teams
  # skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "admins" {
  scope                = azurerm_management_group.management_groups[each.key].id
  principal_id         = azuread_group.admins[each.key].object_id
  role_definition_name = data.azurerm_role_definition.user_access_administrator.name

  for_each = local.teams
  # skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "self_storage_blob_owner" {
  scope                = azurerm_storage_account.backend.id
  principal_id         = azuread_service_principal.administrator.object_id
  role_definition_name = data.azurerm_role_definition.storage_blob_data_owner.name
  # skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "teams" {
  scope                = azurerm_storage_container.container.resource_manager_id
  principal_id         = azuread_group.admins[each.value.slug].object_id
  role_definition_name = data.azurerm_role_definition.storage_blob_data_owner.name

  condition_version = "2.0"
  condition         = <<-EOT
  (
    ${local.write_operations}
    OR 
    (
      %{for repository in each.value.repositories}
        @Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path] StringStartsWith 'github.com/${local.configuration.admin.github.owner}/${repository}'
        OR
      %{endfor~}
      @Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path] StringStartsWith 'github.com/${local.configuration.admin.github.owner}/~/EOT'
    )
  )
  EOT

  for_each = local.role_abac_teams
  # skip_service_principal_aad_check = true
}

resource "azurerm_role_assignment" "appregg" {
  scope                = azurerm_storage_container.container.resource_manager_id
  principal_id         = azuread_service_principal.team[each.key].object_id
  role_definition_name = data.azurerm_role_definition.storage_blob_data_owner.name

  condition_version = "2.0"
  condition         = <<-EOT
  (
   ${local.write_operations}
   OR 
   (
    %{for scope in each.value.scopes}
    @Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path] StringStartsWith 'github.com/${local.configuration.admin.github.owner}/${each.value.repository}/environments/${scope.environment}'
    OR
    %{endfor~}
    @Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path] StringStartsWith 'github.com/${local.configuration.admin.github.owner}/~/EOT'
   )
  )
  EOT

  for_each = local.role_abac_apps
  # skip_service_principal_aad_check = true
}
