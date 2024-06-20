terraform {
  required_providers {
    azuread = {
      source  = "hashicorp/azuread"
      version = "~> 2.48.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">=3.7.0"
    }
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
  }

  # backend "azurerm" {
  #   use_azuread_auth = true
  # }
}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs
provider "azurerm" {
  features {} # Required
}

# https://registry.terraform.io/providers/integrations/github/latest/docs
provider "github" {
  owner = local.configuration.admin.github.owner
  app_auth {} # Required
}

// Current ARM Subscription
data "azurerm_subscription" "current" {}

// Current Callee
data "azuread_client_config" "current" {}

data "azuread_directory_object" "current" {
  object_id = data.azuread_client_config.current.object_id
}

data "azuread_service_principal" "current" {
  object_id = data.azuread_directory_object.current.object_id
  count     = data.azuread_directory_object.current.type == "ServicePrincipal" ? 1 : 0
}

data "azuread_user" "current" {
  object_id = data.azuread_directory_object.current.object_id
  count     = data.azuread_directory_object.current.type == "User" ? 1 : 0
}

resource "terraform_data" "createdBy" {
  input = local.principal
  lifecycle {
    ignore_changes = [input]
  }
}

resource "terraform_data" "createdAt" {
  input = local.timestamp
  lifecycle {
    ignore_changes = [input]
  }
}

locals {
  configuration = yamldecode(file(var.configuration_file))
  principal     = length(data.azuread_service_principal.current) > 0 ? data.azuread_service_principal.current[0].display_name : data.azuread_user.current[0].display_name
  repository    = "github.com/${local.configuration.admin.github.owner}/${local.configuration.admin.github.repository}"

  timestamp = formatdate("DD-MM-YYYY hh:mm:ss ZZZ", timestamp())

  default_tags = {
    "createdAt"  = terraform_data.createdAt.input
    "createdBy"  = terraform_data.createdBy.input
    "modifiedAt" = local.timestamp
    "modifiedBy" = local.principal
    "repository" = local.repository
  }

  oidc_branch = { for oidc in flatten([for product in local.configuration.products :
    [
      for environment in coalesce(var.environments, []) :
      [
        for repository in coalesce(product.repositories, []) :
        {
          slug : "${product.slug}-${environment.name}-${repository}"
          product : product
          repository = repository
        }
      ]
    ]
  ]) : oidc.slug => oidc }

  app_reggs = { for app in flatten([for product in local.configuration.products :
    [
      for environment in coalesce(var.environments, []) :
      [
        for repository in coalesce(product.repositories, []) :
        {
          slug : "${product.slug}-${environment.name}-${repository}"
          product_slug : "${product.slug}-${environment.name}"
          repository : repository
          environment : environment
          product : product
        }
      ]
    ]
  ]) : app.slug => app }

  oidc_environments = { for oidc in flatten([for product in local.configuration.products :
    [
      for environment in coalesce(var.environments, []) :
      [
        for workspace in coalesce(environment.workspaces, []) :
        [
          for workspace_name in coalesce(workspace.names, []) :
          [
            for repository in coalesce(product.repositories, []) :
            {
              slug : "${product.slug}-${workspace_name}-${repository}"
              app_reggs_slug : "${product.slug}-${environment.name}-${repository}"
              repository_name : repository
              environment_name = workspace_name
            }
          ]
        ]
      ]
    ]
  ]) : oidc.slug => oidc }

  role_abac_products = { for product in flatten([for product in local.configuration.products :
    [
      for environment in var.environments :
      [
        {
          slug : "${product.slug}-${environment.name}"
          repositories : coalesce(product.repositories, [])
        }
      ]
    ]
  ]) : product.slug => product }

  role_abac_apps = { for app in flatten([for product in local.configuration.products :
    [
      for environment in coalesce(var.environments, []) :
      [
        for repository in coalesce(product.repositories, []) :
        {
          slug : "${product.slug}-${environment.name}-${repository}"
          repository : repository
          environment : environment
          product : product
          scopes : flatten([for workspace in environment.workspaces :
            [
              for workspace_name in workspace.names :
              {
                environment : workspace_name
              }
            ]
          ])
        }
      ]
    ]
  ]) : app.slug => app }

  products = { for product in flatten([for product in local.configuration.products :
    [
      for environment in var.environments :
      {
        slug : "${product.slug}-${environment.name}"
        product : product
        environment : environment
      }
    ]
  ]) : product.slug => product }
}

