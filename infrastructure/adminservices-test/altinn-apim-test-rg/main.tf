terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
  }
  backend "azurerm" {
    use_azuread_auth = true
  }
}

provider "azurerm" {
  subscription_id = var.subscription_id
  features {}
  resource_providers_to_register = [
    "Microsoft.ApiManagement",
  ]
}

provider "azurerm" {
  alias                           = "adminservices-prod"
  resource_provider_registrations = "none"
  subscription_id                 = var.admin_services_prod_subscription_id
  use_oidc                        = true
  features {}
}