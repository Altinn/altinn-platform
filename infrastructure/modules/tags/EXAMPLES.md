# Tags Module Examples

This document provides comprehensive examples for using the Tags Terraform module.

## Table of Contents

- [Basic Usage](#basic-usage)
  - [Simple Example](#simple-example)
  - [Complete Example with Multiple Resources](#complete-example-with-multiple-resources)
- [Capacity Calculation Examples](#capacity-calculation-examples)
  - [Calculating Capacity from Node Pools](#calculating-capacity-from-node-pools)
  - [Mixed Resource Capacity Calculation](#mixed-resource-capacity-calculation)
- [Organization Number Examples](#organization-number-examples)
  - [Automatic Lookup (Recommended)](#automatic-lookup-recommended)
  - [With Organization Number Override](#with-organization-number-override)
- [User/Principal Identification Patterns](#userprincipal-identification-patterns)
  - [Azure Service Principal (CI/CD)](#azure-service-principal-cicd)
  - [Named Service Principal](#named-service-principal)
  - [Examples of Good Identity Values](#examples-of-good-identity-values)
- [Lifecycle Management Examples](#lifecycle-management-examples)
  - [Consistent Lifecycle Rules Across Resources](#consistent-lifecycle-rules-across-resources)
  - [Module Wrapper for Consistent Tagging](#module-wrapper-for-consistent-tagging)
  - [CI/CD Pipeline with Consistent Identity](#cicd-pipeline-with-consistent-identity)
- [Environment-Specific Examples](#environment-specific-examples)
  - [Development Environment](#development-environment)
  - [Production Environment](#production-environment)
  - [Test Environment](#test-environment)
- [Complete Real-World Example](#complete-real-world-example)

## Basic Usage

### Simple Example

```hcl
# Get current Azure client configuration
data "azurerm_client_config" "current" {}

module "tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  capacity_values = {
    webapp   = 4
    database = 2
  }
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

resource "azurerm_storage_account" "example" {
  name                     = "mystorageaccount"
  resource_group_name      = azurerm_resource_group.rg.name
  location                 = azurerm_resource_group.rg.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  tags                     = module.tags.tags

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}
```

### Complete Example with Multiple Resources

```hcl
# Get current Azure client configuration
data "azurerm_client_config" "current" {}

module "tags" {
  source                  = "./modules/tags"
  finops_environment      = "dev"
  finops_product          = "studio"
  finops_serviceownercode = "skd"
  capacity_values = {
    app_service = 2
    functions   = 1
    containers  = 4
  }
  repository   = "github.com/altinn/altinn-studio"
  current_user = data.azurerm_client_config.current.client_id
}

resource "azurerm_resource_group" "main" {
  name     = "rg-studio-dev"
  location = "Norway East"
  tags     = module.tags.tags

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

resource "azurerm_app_service_plan" "main" {
  name                = "plan-studio-dev"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  
  sku {
    tier = "Standard"
    size = "S1"
  }
  
  tags = module.tags.tags

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}
```

## Capacity Calculation Examples

### Calculating Capacity from Node Pools

For AKS clusters or other scenarios where you need to calculate capacity from multiple pools:

```hcl
# Define your pool configurations
variable "pool_configs" {
  default = {
    syspool = {
      vm_size   = "standard_b4s_v2"
      max_count = 3
    }
    workpool = {
      vm_size   = "standard_b4s_v2"
      max_count = 6
    }
  }
}

# Create mapping of VM sizes to vCPUs (business logic)
locals {
  vm_size_to_vcpus = {
    "standard_b1s_v2"  = 1
    "standard_b2s_v2"  = 2
    "standard_b4s_v2"  = 4
    "standard_b8s_v2"  = 8
    "standard_b16s_v2" = 16
    "standard_b32s_v2" = 32
    # Add more VM sizes as needed
  }
  
  # Calculate capacity for each pool
  pool_capacities = {
    for pool_name, pool in var.pool_configs :
    pool_name => pool.max_count * local.vm_size_to_vcpus[lower(pool.vm_size)]
  }
}

# Get current Azure client configuration
data "azurerm_client_config" "current" {}

module "tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  capacity_values         = local.pool_capacities  # { syspool = 12, workpool = 24 }
  repository              = "github.com/altinn/dialogporten"
  current_user            = data.azurerm_client_config.current.client_id
}

# Result: finops_capacity tag = "36vcpu" (12 + 24)
```

### Mixed Resource Capacity Calculation

For environments with different types of resources:

```hcl
locals {
  # Calculate capacity from different resource types
  capacity_breakdown = {
    aks_cluster  = 36  # Calculated from node pools above
    app_services = 8   # 4 instances × 2 vCPUs each
    function_apps = 2  # 2 dedicated function apps
    sql_database = 4   # DTU converted to approximate vCPU equivalent
  }
}

# Get current Azure client configuration
data "azurerm_client_config" "current" {}

module "tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "studio"
  finops_serviceownercode = "skd"
  capacity_values         = local.capacity_breakdown
  repository              = "github.com/altinn/altinn-studio"
  current_user            = data.azurerm_client_config.current.client_id
}

# Result: finops_capacity tag = "50vcpu" (36 + 8 + 2 + 4)
```

## Organization Number Examples

### Automatic Lookup (Recommended)

```hcl
# Get current Azure client configuration
data "azurerm_client_config" "current" {}

module "tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"  # Will automatically resolve to "974761076"
  capacity_values = {
    webapp = 4
  }
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

resource "azurerm_storage_account" "example" {
  name                     = "mystorageaccount"
  resource_group_name      = azurerm_resource_group.rg.name
  location                 = azurerm_resource_group.rg.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  tags                     = module.tags.tags

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

# Results:
# finops_serviceownerorgnr = "974761076" (automatically looked up)
# organization_name = "Skatteetaten"
# createdby = current_user
# modifiedby = current_user
```

### With Organization Number Override

```hcl
data "azurerm_client_config" "current" {}

module "tags" {
  source                   = "./modules/tags"
  finops_environment       = "prod"
  finops_product           = "dialogporten"
  finops_serviceownercode  = "skd"
  finops_serviceownerorgnr = "123456789"  # Manual override
  capacity_values = {
    webapp = 4
  }
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

# Results:
# finops_serviceownerorgnr = "123456789" (provided override)
# organization_name = null (not available when overriding)
# createdby = current_user
# modifiedby = current_user
```

## User/Principal Identification Patterns

### Azure Service Principal (CI/CD)

```hcl
data "azurerm_client_config" "current" {}

module "tags" {
  source                  = "./modules/tags"
  finops_serviceownercode = "skd"
  current_user            = data.azurerm_client_config.current.client_id
  # ... other variables
}
# Both createdby and modifiedby set to service principal ID
```

### Named Service Principal

```hcl
module "tags" {
  source                  = "./modules/tags"
  finops_serviceownercode = "skd"
  current_user            = "azure-devops-deployment-sp"
  # ... other variables
}
# Both createdby and modifiedby set to named service principal
```

### Examples of Good Identity Values

```hcl
# Good examples for current_user (meaningful, specific identities):
current_user = "john.doe@altinn.no"              # User email
current_user = "deployment-service-principal"    # Service principal name
current_user = "terraform-bootstrap-script"      # Application/script name
current_user = "initial-setup-pipeline"          # Pipeline name
current_user = "azure-devops-sp"                 # Pipeline service principal
current_user = "maintenance-automation"          # Automated system
current_user = "emergency-response-team"         # Team identifier

# Bad examples (too generic, but still allowed):
current_user = "terraform"     # Generic but allowed
current_user = "system"        # Generic but allowed
```

## Lifecycle Management Examples

### Consistent Lifecycle Rules Across Resources

```hcl
locals {
  # Define common lifecycle rule for all tagged resources
  tag_lifecycle = {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

resource "azurerm_resource_group" "main" {
  # ... configuration
  tags = module.tags.tags
  
  lifecycle {
    ignore_changes = local.tag_lifecycle.ignore_changes
  }
}

resource "azurerm_storage_account" "example" {
  # ... configuration
  tags = module.tags.tags
  
  lifecycle {
    ignore_changes = local.tag_lifecycle.ignore_changes
  }
}
```

### Module Wrapper for Consistent Tagging

```hcl
# Create a wrapper module that includes lifecycle rules
module "tagged_storage_account" {
  source = "./modules/tagged-storage-account"
  
  # Storage account parameters
  name = "mystorageaccount"
  # ... other parameters
  
  # Tag parameters
  current_user = data.azurerm_client_config.current.client_id
  # ... other tag parameters
}

# In the wrapper module (modules/tagged-storage-account/main.tf):
resource "azurerm_storage_account" "this" {
  # ... configuration
  tags = module.tags.tags
  
  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}
```

### CI/CD Pipeline with Consistent Identity

```hcl
# Use consistent service principal name across all resources
module "tags" {
  source       = "./modules/tags"
  current_user = "azure-devops-deployment-sp"
  # ... other variables
}
```

## Environment-Specific Examples

### Development Environment

```hcl
module "tags" {
  source                  = "./modules/tags"
  finops_environment      = "dev"
  finops_product          = "studio"
  finops_serviceownercode = "skd"
  capacity_values = {
    dev_resources = 2
  }
  repository   = "github.com/altinn/altinn-studio"
  current_user = "developer.name@altinn.no"
}
```

### Production Environment

```hcl
module "tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  capacity_values = {
    web_tier     = 16
    app_tier     = 32
    data_tier    = 8
    cache_tier   = 4
  }
  repository   = "github.com/altinn/dialogporten"
  current_user = "production-deployment-sp"
}
```

### Test Environment

```hcl
module "tags" {
  source                  = "./modules/tags"
  finops_environment      = "at24"
  finops_product          = "formidling"
  finops_serviceownercode = "digdir"
  capacity_values = {
    test_load = 4
  }
  repository   = "github.com/altinn/altinn-formidling"
  current_user = "test-automation-sp"
}
```

## Complete Real-World Example

```hcl
# Get current Azure client configuration
data "azurerm_client_config" "current" {}

# Define VM size to vCPU mapping
locals {
  vm_size_to_vcpus = {
    "standard_b4s_v2"  = 4
    "standard_d4s_v3"  = 4
    "standard_d8s_v3"  = 8
  }
  
  # Calculate AKS capacity
  aks_capacity = {
    system_pool = 3 * local.vm_size_to_vcpus["standard_b4s_v2"]  # 12 vCPUs
    user_pool   = 5 * local.vm_size_to_vcpus["standard_d4s_v3"]  # 20 vCPUs
  }
  
  # Other service capacities
  other_capacity = {
    app_services    = 8   # 4 instances × 2 vCPUs
    function_apps   = 2   # 2 consumption plan apps
    sql_databases   = 4   # DTU equivalent
  }
  
  # Total capacity
  total_capacity = merge(local.aks_capacity, local.other_capacity)
}

# Tags module
module "tags" {
  source                  = "./modules/tags"
  finops_environment      = var.environment
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  capacity_values         = local.total_capacity
  repository              = "github.com/altinn/dialogporten"
  current_user            = data.azurerm_client_config.current.client_id
}

# Resource group
resource "azurerm_resource_group" "main" {
  name     = "rg-dialogporten-${var.environment}"
  location = "Norway East"
  tags     = module.tags.tags

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

# AKS cluster
resource "azurerm_kubernetes_cluster" "main" {
  name                = "aks-dialogporten-${var.environment}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  dns_prefix          = "aks-dialogporten-${var.environment}"

  default_node_pool {
    name       = "system"
    node_count = 3
    vm_size    = "Standard_B4s_v2"
  }

  identity {
    type = "SystemAssigned"
  }

  tags = module.tags.tags

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

# Additional node pool
resource "azurerm_kubernetes_cluster_node_pool" "user" {
  name                  = "user"
  kubernetes_cluster_id = azurerm_kubernetes_cluster.main.id
  vm_size              = "Standard_D4s_v3"
  node_count           = 5

  tags = module.tags.tags

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

# Output the generated tags for verification
output "generated_tags" {
  description = "All generated tags for verification"
  value       = module.tags.tags
}

output "total_vcpus" {
  description = "Total calculated vCPU capacity"
  value       = module.tags.total_vcpus
}

output "capacity_breakdown" {
  description = "Breakdown of capacity calculation"
  value       = module.tags.capacity_breakdown
}
```

This example demonstrates:
- Capacity calculation from multiple sources (AKS node pools, app services, etc.)
- Proper lifecycle rules for immutability
- Real-world resource configuration
- Output values for verification
- Dynamic environment handling
- Service principal authentication