terraform {
  required_providers {
    azuread = {
      source  = "hashicorp/azuread"
      version = "~> 2.48.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.0.0"
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

locals {
  configuration = yamldecode(file(var.configuration_file))

  oidc_branch = { for oidc in flatten([for team in local.configuration.teams :
    [
      for environment in coalesce(var.environments, []) :
      [
        for repository in coalesce(team.repositories, []) :
        {
          slug : "${team.slug}-${environment.name}-${repository}"
          team : team
          repository = repository
        }
      ]
    ]
  ]) : oidc.slug => oidc }

  app_reggs = { for app in flatten([for team in local.configuration.teams :
    [
      for environment in coalesce(var.environments, []) :
      [
        for repository in coalesce(team.repositories, []) :
        {
          slug : "${team.slug}-${environment.name}-${repository}"
          repository : repository
          environment : environment
          team : team
        }
      ]
    ]
  ]) : app.slug => app }

  oidc_environments = { for oidc in flatten([for team in local.configuration.teams :
    [
      for environment in coalesce(var.environments, []) :
      [
        for workspace in coalesce(environment.workspaces, []) :
        [
          for workspace_name in coalesce(workspace.names, []) :
          [
            for repository in coalesce(team.repositories, []) :
            {
              slug : "${team.slug}-${workspace_name}-${repository}"
              app_reggs_slug : "${team.slug}-${environment.name}-${repository}"
              repository_name : repository
              environment_name = workspace_name
            }
          ]
        ]
      ]
    ]
  ]) : oidc.slug => oidc }

  role_abac_teams = { for team in flatten([for team in local.configuration.teams :
    [
      for environment in var.environments :
      [
        {
          slug : "${team.slug}-${environment.name}"
          repositories : coalesce(team.repositories, [])
        }
      ]
    ]
  ]) : team.slug => team }

  role_abac_apps = { for app in flatten([for team in local.configuration.teams :
    [
      for environment in coalesce(var.environments, []) :
      [
        for repository in coalesce(team.repositories, []) :
        {
          slug : "${team.slug}-${environment.name}-${repository}"
          repository : repository
          environment : environment
          team : team
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

  teams = { for team in flatten([for team in local.configuration.teams :
    [
      for environment in var.environments :
      {
        slug : "${team.slug}-${environment.name}"
        team : team
        environment : environment
      }
    ]
  ]) : team.slug => team }
}

