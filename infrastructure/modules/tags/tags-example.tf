# Example: Using the Altinn Platform tags.tf in your Terraform project

terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.0"
    }
    http = {
      source  = "hashicorp/http"
      version = "~> 3.4"
    }
  }
}

provider "azurerm" {
  features {}
}

# Example terraform.tfvars values:
# finops_environment      = "prod"
# finops_product          = "dialogporten"
# finops_serviceownercode = "skd"  # Optional
# repository              = "github.com/altinn/dialogporten"  # Optional
# current_user            = "terraform-sp"
# created_date            = "2024-03-15"  # Optional
# modified_date           = ""  # Optional

# Resource Group
resource "azurerm_resource_group" "main" {
  name     = "rg-${var.finops_product}-${var.finops_environment}"
  location = "Norway East"

  tags = merge(local.base_tags, {
    providedby = "teamname"
  })

  lifecycle {
    ignore_changes = [tags["createdby"], tags["createddate"]]
  }
}

# AKS Cluster
resource "azurerm_kubernetes_cluster" "main" {
  name                = "aks-${var.finops_product}-${var.finops_environment}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  dns_prefix          = "aks-${var.finops_product}"

  default_node_pool {
    name       = "system"
    node_count = 3
    vm_size    = "Standard_D4s_v3"
  }

  identity {
    type = "SystemAssigned"
  }

  tags = merge(local.base_tags, {
    providedby = "teamname"
  })

  lifecycle {
    ignore_changes = [tags["createdby"], tags["createddate"]]
  }
}

# PostgreSQL
resource "azurerm_postgresql_flexible_server" "main" {
  name                = "psql-${var.finops_product}-${var.finops_environment}"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  version             = "15"
  sku_name            = "GP_Standard_D2s_v3"

  tags = merge(local.base_tags, {
    providedby = "teamname"
  })

  lifecycle {
    ignore_changes = [tags["createdby"], tags["createddate"]]
  }
}

# Storage Account
resource "azurerm_storage_account" "main" {
  name                     = "st${replace(var.finops_product, "-", "")}${var.finops_environment}"
  resource_group_name      = azurerm_resource_group.main.name
  location                 = azurerm_resource_group.main.location
  account_tier             = "Standard"
  account_replication_type = "LRS"

  tags = merge(local.base_tags, {
    providedby = "teamname"
  })

  lifecycle {
    ignore_changes = [tags["createdby"], tags["createddate"]]
  }
}

# Key Vault
resource "azurerm_key_vault" "main" {
  name                = "kv-${var.finops_product}-${var.finops_environment}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  tenant_id           = data.azurerm_client_config.current.tenant_id
  sku_name            = "standard"

  tags = merge(local.base_tags, {
    providedby = "teamname"
  })

  lifecycle {
    ignore_changes = [tags["createdby"], tags["createddate"]]
  }
}

data "azurerm_client_config" "current" {}

# Outputs for verification
output "tags_example" {
  description = "Example tags for all resources"
  value       = local.base_tags
}
