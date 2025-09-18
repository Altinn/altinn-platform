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
    time = {
      source = "hashicorp/time"
    }
    random = {
      source = "hashicorp/random"
    }
    postgresql = {
      source  = "cyrilgdn/postgresql"
      version = "__postgresql_provider__"
    }
    azapi = {
      source  = "azure/azapi"
      version = "~> 1.13"
    }
  }
}

