terraform {
  required_providers {
    azurerm = {
      source = "hashicorp/azurerm"
    }
    azuread = {
      source = "hashicorp/azuread"
    }
    null = {
      source = "hashicorp/null"
    }
    azapi = {
      source  = "azure/azapi"
      version = "~> 1.13"
    }
  }
}