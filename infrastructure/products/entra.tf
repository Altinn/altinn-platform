#
# To reference the app registrations in ARM, use following reference azuread_service_principal.product[<product_name>].object_id
#

resource "azuread_application" "administrator" {
  display_name = "GitHub: ${lower(local.configuration.admin.github.owner)}/${lower(local.configuration.admin.github.repository)} - Admin"
  #  prevent_duplicate_names = true
  lifecycle { create_before_destroy = false }
}

resource "azuread_service_principal" "administrator" {
  client_id = azuread_application.administrator.client_id
}

resource "azuread_application" "product" {
  display_name = "GitHub: ${lower(local.configuration.admin.github.owner)}/${each.value.repository} - ${title(each.value.environment.name)}"
  #  prevent_duplicate_names = true

  for_each = local.app_reggs
  lifecycle { create_before_destroy = false }
}

resource "azuread_service_principal" "product" {
  client_id = azuread_application.product[each.key].client_id
  for_each  = local.app_reggs
}

resource "azuread_group" "readers" {
  display_name     = "Altinn Product ${each.value.product.name}: Readers ${title(each.value.environment.name)}"
  security_enabled = true

  for_each = local.products
}

resource "azuread_group" "developers" {
  display_name     = "Altinn Product ${each.value.product.name}: Developers ${title(each.value.environment.name)}"
  security_enabled = true

  for_each = local.products
}

resource "azuread_group" "admins" {
  display_name     = "Altinn Product ${each.value.product.name}: Admins ${title(each.value.environment.name)}"
  security_enabled = true

  for_each = local.products
}

resource "azuread_group" "terraform_admins" {
  display_name     = "Altinn Products Terraform: Admins"
  security_enabled = true
}

resource "azuread_group_member" "terraform_admins" {
  group_object_id  = azuread_group.terraform_admins.object_id
  member_object_id = data.azuread_client_config.current.object_id
}

resource "azuread_group_member" "admin_contributor" {
  group_object_id  = azuread_group.developers[each.key].id
  member_object_id = azuread_group.admins[each.key].object_id
  for_each         = local.products
}

resource "azuread_group_member" "contributor_reader" {
  group_object_id  = azuread_group.readers[each.key].id
  member_object_id = azuread_group.developers[each.key].object_id
  for_each         = local.products
}

resource "azuread_application_federated_identity_credential" "oidc_environments" {
  application_id = azuread_application.product[each.value.app_reggs_slug].id
  display_name   = "github.${lower(local.configuration.admin.github.owner)}.${each.value.repository_name}.environment.${each.value.environment_name}"
  subject        = "repo:${lower(local.configuration.admin.github.owner)}/${each.value.repository_name}:environment:${each.value.environment_name}"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = "https://token.actions.githubusercontent.com"
  description    = "Allow GitHub actions run within the context of environment '${each.value.environment_name}' from the repository https://github.com/${local.configuration.admin.github.owner}/${each.value.repository_name} to have access to the app registration"

  for_each = local.oidc_environments
}

resource "azuread_application_federated_identity_credential" "oidc_branch" {
  application_id = azuread_application.product[each.key].id
  display_name   = "github.${lower(local.configuration.admin.github.owner)}.${each.value.repository}.branch.main"
  subject        = "repo:${local.configuration.admin.github.owner}/${each.value.repository}:ref:refs/heads/main"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = "https://token.actions.githubusercontent.com"
  description    = "Allow GitHub actions run within the context of branch 'main' from the repository https://github.com/${local.configuration.admin.github.owner}/${each.value.repository} to have access to the app registration"

  for_each = local.oidc_branch
}
