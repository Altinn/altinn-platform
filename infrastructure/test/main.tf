terraform {
  required_providers {
    azuread = {
      source  = "hashicorp/azuread"
      version = "~> 2.48.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">=3.108.0"
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


resource "azurerm_resource_group" "rg" {
  name     = "altinnkake2"
  location = "norwayeast"
}
