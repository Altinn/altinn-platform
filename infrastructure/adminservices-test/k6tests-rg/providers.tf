terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "3.7.1"
    }
  }

  backend "azurerm" {
    use_azuread_auth = true
  }

}

provider "azurerm" {
  features {}
}

provider "helm" {
  kubernetes {
    config_path    = "~/.kube/config"
    config_context = "k6tests-cluster"
  }
}

provider "kubernetes" {
  config_path    = "~/.kube/config"
  config_context = "k6tests-cluster"
}

provider "random" {}
