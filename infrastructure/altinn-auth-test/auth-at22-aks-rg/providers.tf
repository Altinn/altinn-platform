terraform {
  required_providers {
    azapi = {
      source  = "Azure/azapi"
      version = "~> 2.0"
    }
    azuread = {
      source  = "hashicorp/azuread"
      version = "~> 3.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
    http = {
      source  = "hashicorp/http"
      version = "~> 3.0"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.13"
    }
    grafana = {
      source  = "grafana/grafana"
      version = "~> 3.0"
    }
  }
  backend "azurerm" {
    use_azuread_auth = true
  }
}

provider "azapi" {
  subscription_id        = var.subscription_id
  use_oidc               = true
  enable_preflight       = true
  disable_default_output = true
}

provider "azuread" {
  use_oidc = true
}

provider "azurerm" {
  features {}
  subscription_id     = var.subscription_id
  use_oidc            = true
  storage_use_azuread = true
  resource_providers_to_register = [
    "Microsoft.Monitor",
    "Microsoft.AlertsManagement",
    "Microsoft.Dashboard",
    "Microsoft.KubernetesConfiguration"
  ]
}

provider "grafana" {
  url  = module.grafana.grafana_endpoint
  auth = var.app_access_token
}

provider "random" {}
provider "http" {}
provider "time" {}
