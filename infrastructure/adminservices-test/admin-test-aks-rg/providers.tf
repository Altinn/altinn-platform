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

provider "kubectl" {
  load_config_file       = false
  host                   = module.aks.kube_config.0.host
  cluster_ca_certificate = base64decode(module.aks.kube_config.0.cluster_ca_certificate)
  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "az"
    args = [
      "aks",
      "get-credentials",
      "--resource-group", "admin-test-monitor-rg",
      "--name", "your-aks-cluster-name",
      "--overwrite-existing"
    ]
  }
}

provider "azapi" {
  subscription_id        = var.subscription_id
  use_oidc               = true
  enable_preflight       = var.arm_enable_preflight
  disable_default_output = true
}
