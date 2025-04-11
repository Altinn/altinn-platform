terraform {
  required_providers {
    azurerm = {
      source = "hashicorp/azurerm"
    }
    azapi = {
      source  = "Azure/azapi"
      version = ">= 2.3.0"
    }
  }
}
