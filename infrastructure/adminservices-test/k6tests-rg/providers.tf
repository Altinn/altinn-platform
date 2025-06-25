terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "3.7.2"
    }
  }

  backend "azurerm" {}

}

provider "azurerm" {
  features {}
}

provider "helm" {
  kubernetes = {
    config_path    = "~/.kube/config"
    config_context = "k6tests-cluster"
  }
}

provider "kubernetes" {
  config_path    = "~/.kube/config"
  config_context = "k6tests-cluster"
}

provider "random" {}
