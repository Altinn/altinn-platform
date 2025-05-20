terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.0.0"
    }
    grafana = {
      source  = "grafana/grafana"
      version = ">= 3.0.0"
    }
  }
}
