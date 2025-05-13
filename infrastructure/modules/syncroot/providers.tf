terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0" # Any version >= 4.0.0 < 5.0.0
    }
    azapi = {
      source  = "Azure/azapi"
      version = "~> 2.0" # Any version >= 2.0.0 < 3.0.0
    }
  }
}
