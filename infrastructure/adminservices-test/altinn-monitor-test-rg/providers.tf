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
  subscription_id = "1ce8e9af-c2d6-44e7-9c5e-099a308056fe"
  features {}
  resource_providers_to_register = [
    "Microsoft.Monitor",
    "Microsoft.AlertsManagement",
    "Microsoft.Dashboard",
    "Microsoft.KubernetesConfiguration"
  ]
}

# Dialogporten
provider "azurerm" {
  alias           = "dp_test"
  subscription_id = "8a353de8-d81d-468d-a40d-f3574b6bb3f4"
  features {}
  resource_provider_registrations = "none"
}

provider "azurerm" {
  alias           = "dp_stag"
  subscription_id = "e4926efc-0577-47b3-9c3d-757925630eca"
  features {}
  resource_provider_registrations = "none"
}

provider "azurerm" {
  alias           = "dp_prod"
  subscription_id = "c595f787-450d-4c57-84fa-abc5f95d5459"
  features {}
  resource_provider_registrations = "none"
}

# Studio
provider "azurerm" {
  alias           = "studio_test"
  subscription_id = "971ddbb1-27d0-4cc7-a016-461dab5cec05"
  features {}
  resource_provider_registrations = "none"
}

provider "azurerm" {
  alias           = "studio_prod"
  subscription_id = "f66298ed-870c-40e0-bb74-6db89c1a364b"
  features {}
  resource_provider_registrations = "none"
}
