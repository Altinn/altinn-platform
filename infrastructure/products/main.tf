terraform {
  required_providers {
    azuread = {
      source  = "hashicorp/azuread"
      version = "~> 3.0.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 3.7.0"
    }
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
  }

  backend "azurerm" {
    use_azuread_auth = true
  }
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

resource "terraform_data" "created_by" {
  input = local.principal
  lifecycle {
    ignore_changes = [input]
  }
}

resource "terraform_data" "created_at" {
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
    "createdAt"  = terraform_data.created_at.input
    "createdBy"  = terraform_data.created_by.input
    "modifiedAt" = local.timestamp
    "modifiedBy" = local.principal
    "repository" = local.repository
  }

  oidc_branch = { for oidc in flatten([for product in local.configuration.products :
    [
      for workspace in coalesce(var.workspaces, []) :
      [
        for repository in coalesce(product.github.repositories, []) :
        {
          slug : lower("${product.slug}-${product.github.owner}-${repository}-${workspace.name}")
          product : product

          repository : {
            owner : product.github.owner
            name : repository
          }
        }
      ]
    ]
  ]) : oidc.slug => oidc }

  app_reggs = { for app in flatten([for product in local.configuration.products :
    [
      for workspace in coalesce(var.workspaces, []) :
      [
        for repository in coalesce(product.github.repositories, []) :
        {
          slug : lower("${product.slug}-${product.github.owner}-${repository}-${workspace.name}")
          product_slug : lower("${product.slug}-${workspace.name}")
          product : product
          workspace : workspace

          repository : {
            owner : product.github.owner
            name : repository
          }
        }
      ]
    ]
  ]) : app.slug => app }

  oidc_environments = { for oidc in flatten([for product in local.configuration.products :
    [
      for workspace in coalesce(var.workspaces, []) :
      [
        for environment in coalesce(workspace.environments, []) :
        [
          for environment_name in coalesce(environment.names, []) :
          [
            for repository in coalesce(product.github.repositories, []) :
            {
              slug : lower("${product.slug}-${product.github.owner}-${repository}-${workspace.name}-${environment_name}")
              app_reggs_slug : lower("${product.slug}-${product.github.owner}-${repository}-${workspace.name}")

              environment : {
                name : environment_name
              }

              repository : {
                owner : product.github.owner
                name = repository
              }
            }
          ]
        ]
      ]
    ]
  ]) : oidc.slug => oidc }

  role_abac_products = { for product in flatten([for product in local.configuration.products :
    [
      for workspace in coalesce(var.workspaces, []) :
      [
        {
          slug : lower("${product.slug}-${workspace.name}")

          repositories : {
            owner : product.github.owner
            names = coalesce(product.github.repositories, [])
          }
        }
      ]
    ]
  ]) : product.slug => product }

  role_abac_apps = { for app in flatten([for product in local.configuration.products :
    [
      for workspace in coalesce(var.workspaces, []) :
      [
        for repository in coalesce(product.github.repositories, []) :
        {
          slug : lower("${product.slug}-${product.github.owner}-${repository}-${workspace.name}")
          product : product

          repository : {
            owner : product.github.owner
            name = repository
          }
          scopes : flatten([for environment in workspace.environments :
            [
              for environment_name in environment.names :
              {
                environment : {
                  name : environment_name
                }
              }
            ]
          ])
        }
      ]
    ]
  ]) : app.slug => app }

  products = { for product in flatten([for product in local.configuration.products :
    [
      for workspace in var.workspaces :
      {
        slug : lower("${product.slug}-${workspace.name}")
        product : product
        workspace : workspace
      }
    ]
  ]) : product.slug => product }

  environments = { for environment in flatten([for workspace in var.workspaces :
    [
      for environment in workspace.environments :
      [
        for environment_name in environment.names : environment_name
      ]
    ]
  ]) : environment => environment }
}

