#
# To reference the app registrations in ARM, use following reference azuread_service_principal.product[<product_name>].object_id
#

data "azuread_application_published_app_ids" "well_known" {}

data "azuread_service_principal" "msgraph" {
  client_id = data.azuread_application_published_app_ids.well_known.result["MicrosoftGraph"]
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application
resource "azuread_application" "administrator" {
  display_name = "GitHub: ${local.configuration.admin.github.owner}/${lower(local.configuration.admin.github.repository)} - Admin"
  #  prevent_duplicate_names = true
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application
resource "azuread_application" "reader" {
  display_name = "GitHub: ${local.configuration.admin.github.owner}/${lower(local.configuration.admin.github.repository)} - Reader"
  #  prevent_duplicate_names = true
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application
resource "azuread_service_principal" "administrator" {
  client_id = azuread_application.administrator.client_id
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application
resource "azuread_service_principal" "reader" {
  client_id = azuread_application.reader.client_id
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application
resource "azuread_application" "product" {
  display_name = "GitHub: ${lower(each.value.repository.owner)}/${each.value.repository.name} - ${title(each.value.workspace.name)}"
  #  prevent_duplicate_names = true

  for_each = local.app_reggs
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application
resource "azuread_service_principal" "product" {
  client_id = azuread_application.product[each.key].client_id

  for_each = local.app_reggs
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/group
resource "azuread_group" "readers" {
  display_name     = "Altinn Product ${each.value.product.name}: Readers ${title(each.value.workspace.name)}"
  security_enabled = true

  for_each = local.products
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/group
resource "azuread_group" "developers" {
  display_name     = "Altinn Product ${each.value.product.name}: Developers ${title(each.value.workspace.name)}"
  security_enabled = true

  for_each = local.products
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/group
resource "azuread_group" "admins" {
  display_name     = "Altinn Product ${each.value.product.name}: Admins ${title(each.value.workspace.name)}"
  security_enabled = true

  for_each = local.products
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/group
resource "azuread_group" "product_admins" {
  display_name     = "Altinn Products: Admins"
  security_enabled = true
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/group
resource "azuread_group" "product_readers" {
  display_name     = "Altinn Products: Readers"
  security_enabled = true
}

resource "azuread_application_api_access" "example_msgraph" {
  application_id = azuread_application.administrator.id
  api_client_id  = data.azuread_application_published_app_ids.well_known.result["MicrosoftGraph"]

  role_ids = [
    data.azuread_service_principal.msgraph.app_role_ids["Group.ReadWrite.All"],
    data.azuread_service_principal.msgraph.app_role_ids["Application.ReadWrite.All"],
  ]
}

resource "azuread_application_api_access" "reader_msgraph" {
  application_id = azuread_application.reader.id
  api_client_id  = data.azuread_application_published_app_ids.well_known.result["MicrosoftGraph"]

  role_ids = [
    data.azuread_service_principal.msgraph.app_role_ids["Group.Read.All"],
    data.azuread_service_principal.msgraph.app_role_ids["Application.Read.All"],
  ]
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/group_member
resource "azuread_group_member" "product_admins" {
  group_object_id  = azuread_group.product_admins.object_id
  member_object_id = azuread_service_principal.administrator.object_id
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/group_member
resource "azuread_group_member" "product_readers" {
  group_object_id  = azuread_group.product_readers.object_id
  member_object_id = azuread_service_principal.reader.object_id
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/group_member
resource "azuread_group_member" "admin_contributor" {
  group_object_id  = azuread_group.developers[each.key].object_id
  member_object_id = azuread_group.admins[each.key].object_id

  for_each = local.products
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/group_member
resource "azuread_group_member" "contributor_reader" {
  group_object_id  = azuread_group.readers[each.key].object_id
  member_object_id = azuread_group.developers[each.key].object_id

  for_each = local.products
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application_federated_identity_credential
resource "azuread_application_federated_identity_credential" "oidc_environments_admin" {
  application_id = azuread_application.administrator.id
  display_name   = "github.${local.configuration.admin.github.owner}.${local.configuration.admin.github.repository}.environment.${each.value}"
  subject        = "repo:${local.configuration.admin.github.owner}/${lower(local.configuration.admin.github.repository)}:environment:${each.value}"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = "https://token.actions.githubusercontent.com"
  description    = "Allow GitHub actions run within the context of environment '${each.value}' from the repository https://github.com/${local.configuration.admin.github.owner}/${lower(local.configuration.admin.github.repository)} to have access to the app registration"

  for_each = local.environments
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application_federated_identity_credential
resource "azuread_application_federated_identity_credential" "oidc_environments_reader" {
  application_id = azuread_application.reader.id
  display_name   = "github.${local.configuration.admin.github.owner}.${local.configuration.admin.github.repository}.environment.reader"
  subject        = "repo:${local.configuration.admin.github.owner}/${lower(local.configuration.admin.github.repository)}:environment:reader"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = "https://token.actions.githubusercontent.com"
  description    = "Allow GitHub actions run within the context of environment reader from the repository https://github.com/${local.configuration.admin.github.owner}/${lower(local.configuration.admin.github.repository)} to have access to the app registration"
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application_federated_identity_credential
resource "azuread_application_federated_identity_credential" "oidc_environments" {
  application_id = azuread_application.product[each.value.app_reggs_slug].id
  display_name   = "github.${each.value.repository.owner}.${each.value.repository.name}.environment.${each.value.environment.name}"
  subject        = "repo:${each.value.repository.owner}/${each.value.repository.name}:environment:${each.value.environment.name}"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = "https://token.actions.githubusercontent.com"
  description    = "Allow GitHub actions run within the context of environment '${each.value.environment.name}' from the repository https://github.com/${each.value.repository.owner}/${each.value.repository.name} to have access to the app registration"

  for_each = local.oidc_environments
}

# https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application_federated_identity_credential
resource "azuread_application_federated_identity_credential" "oidc_branch" {
  application_id = azuread_application.product[each.key].id
  display_name   = "github.${each.value.repository.owner}.${each.value.repository.name}.branch.main"
  subject        = "repo:${each.value.repository.owner}/${each.value.repository.name}:ref:refs/heads/main"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = "https://token.actions.githubusercontent.com"
  description    = "Allow GitHub actions run within the context of branch 'main' from the repository https://github.com/${each.value.repository.owner}/${each.value.repository.name} to have access to the app registration"

  for_each = local.oidc_branch
}
