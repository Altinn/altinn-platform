
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

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/storage_account
resource "azurerm_storage_account" "backend" {
  name                     = "${var.arm_product_name}${var.arm_solution_name}storage${var.arm_instance}"
  resource_group_name      = data.azurerm_resource_group.tfstate.name
  location                 = data.azurerm_resource_group.tfstate.location
  account_kind             = "BlobStorage"
  access_tier              = "Hot"
  account_tier             = "Standard"
  account_replication_type = "GRS"

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
  name                 = "tfstates"
  storage_account_name = azurerm_storage_account.backend.name
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/api_management_group
resource "azurerm_management_group" "parent" {
  name         = "ALP"
  display_name = "Altinn-Products"
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/api_management_group
resource "azurerm_management_group" "management_groups" {
  name                       = "${each.value.product.slug}-${title(each.value.workspace.name)}"
  display_name               = "Altinn-${replace(each.value.product.name, " ", "-")}-${title(each.value.workspace.name)}"
  parent_management_group_id = azurerm_management_group.parent.id

  for_each = local.products
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/management_group_subscription_association
resource "azurerm_management_group_subscription_association" "subscriptions" {
  management_group_id = azurerm_management_group.management_groups[each.key].id
  subscription_id     = azurerm_subscription.subscriptions[each.key].id

  for_each = { for key, value in local.products : key => value if var.arm_billing_account_name != null && var.arm_enrollment_account_scope != null }
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "administrator_user_access_administrator" {
  scope                            = azurerm_management_group.parent.id
  principal_id                     = azuread_service_principal.administrator.object_id
  role_definition_name             = data.azurerm_role_definition.user_access_administrator.name
  skip_service_principal_aad_check = true
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "administrator_contributor" {
  scope                            = azurerm_management_group.parent.id
  principal_id                     = azuread_service_principal.administrator.object_id
  role_definition_name             = data.azurerm_role_definition.contributor.name
  skip_service_principal_aad_check = true
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "apps_user_access_administrator" {
  scope                            = azurerm_management_group.management_groups[each.value.product_slug].id
  principal_id                     = azuread_service_principal.product[each.key].object_id
  role_definition_name             = data.azurerm_role_definition.user_access_administrator.name
  skip_service_principal_aad_check = true

  for_each = local.app_reggs
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "apps_contributor" {
  scope                            = azurerm_management_group.management_groups[each.value.product_slug].id
  principal_id                     = azuread_service_principal.product[each.key].object_id
  role_definition_name             = data.azurerm_role_definition.contributor.name
  skip_service_principal_aad_check = true

  for_each = local.app_reggs
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "readers" {
  scope                = azurerm_management_group.management_groups[each.key].id
  principal_id         = azuread_group.readers[each.key].object_id
  role_definition_name = data.azurerm_role_definition.reader.name

  for_each = local.products
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "developers" {
  scope                = azurerm_management_group.management_groups[each.key].id
  principal_id         = azuread_group.developers[each.key].object_id
  role_definition_name = data.azurerm_role_definition.contributor.name

  for_each = local.products
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "admins" {
  scope                = azurerm_management_group.management_groups[each.key].id
  principal_id         = azuread_group.admins[each.key].object_id
  role_definition_name = data.azurerm_role_definition.user_access_administrator.name

  for_each = local.products
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment
resource "azurerm_role_assignment" "product_admins_storage_blob_owner" {
  scope                = azurerm_storage_account.backend.id
  principal_id         = azuread_group.product_admins.object_id
  role_definition_name = data.azurerm_role_definition.storage_blob_data_owner.name
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
resource "azurerm_role_assignment" "products" {
  scope                = azurerm_storage_container.container.resource_manager_id
  principal_id         = azuread_group.admins[each.value.slug].object_id
  role_definition_name = data.azurerm_role_definition.storage_blob_data_owner.name

  depends_on = [azurerm_role_assignment.product_admins_contributor]

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
  scope                            = azurerm_storage_container.container.resource_manager_id
  principal_id                     = azuread_service_principal.product[each.key].object_id
  role_definition_name             = data.azurerm_role_definition.storage_blob_data_owner.name
  skip_service_principal_aad_check = true

  depends_on = [azurerm_role_assignment.product_admins_contributor]

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
