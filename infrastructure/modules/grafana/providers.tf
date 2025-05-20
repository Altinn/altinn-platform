terraform {
  required_providers {
    azuread = {
      source  = "hashicorp/azuread"
      version = ">= 3.0.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.0.0"
    }
    grafana = {
      source  = "grafana/grafana"
      version = ">= 3.0.0"
    }
    http = {
      source = "hashicorp/http"
    }
    time = {
      source = "hashicorp/time"
    }
  }
}
