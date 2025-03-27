terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
    azuread = {
      source  = "hashicorp/azuread"
      version = "~> 3.1"
    }
    kubectl = {
      source  = "gavinbunney/kubectl"
      version = "~> 1.19.0"
    }
    azapi = {
      source  = "Azure/azapi"
      version = "~> 2.0"
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
    "Microsoft.Monitor",
    "Microsoft.AlertsManagement",
    "Microsoft.Dashboard",
    "Microsoft.KubernetesConfiguration"
  ]
}

provider "azurerm" {
  alias                           = "adminservices-prod"
  resource_provider_registrations = "none"
  subscription_id                 = var.admin_services_prod_subscription_id
  use_oidc                        = true
  features {}
}

provider "kubectl" {
  load_config_file       = false
  client_certificate     = base64decode(azurerm_kubernetes_cluster.aks.kube_config.0.client_certificate)
  client_key             = base64decode(azurerm_kubernetes_cluster.aks.kube_config.0.client_key)
  host                   = azurerm_kubernetes_cluster.aks.kube_config.0.host
  cluster_ca_certificate = base64decode(azurerm_kubernetes_cluster.aks.kube_config.0.cluster_ca_certificate)
}

provider "azapi" {
  use_oidc         = true
  enable_preflight = true
}
