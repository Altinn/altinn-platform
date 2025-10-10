# Tags Module Examples

This document provides comprehensive examples for using the Tags Terraform module.

## Table of Contents

- [Basic Usage](#basic-usage)
  - [Simple Example](#simple-example)
  - [Complete Example with Multiple Resources](#complete-example-with-multiple-resources)
- [Resource Type Examples](#resource-type-examples)
  - [Computing Resources (with capacity)](#computing-resources-with-capacity)
  - [Non-Computing Resources (without capacity)](#non-computing-resources-without-capacity)
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
- [Error Handling and Debugging Examples](#error-handling-and-debugging-examples)
  - [Service Owner Code Validation](#service-owner-code-validation)
  - [External API Error Handling](#external-api-error-handling)
  - [Debug Information Usage](#debug-information-usage)
- [Complete Real-World Example](#complete-real-world-example)

## Resource Type Examples

### Computing Resources (with capacity)

Computing resources like AKS clusters, VMs, databases, and app services should include capacity information:

```hcl
# AKS Cluster - Computing Resource
data "azurerm_client_config" "current" {}

module "aks_tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  capacity_values = [12, 20]  # system_pool: 12 vCPUs, user_pool: 20 vCPUs
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

resource "azurerm_kubernetes_cluster" "main" {
  name                = "aks-dialogporten-prod"
  location            = "Norway East"
  resource_group_name = azurerm_resource_group.main.name
  dns_prefix          = "aks-dialogporten-prod"

  default_node_pool {
    name       = "system"
    node_count = 3
    vm_size    = "Standard_B4s_v2"
  }

  identity {
    type = "SystemAssigned"
  }

  tags = module.aks_tags.tags  # Includes finops_capacity = "32vcpu"

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}
```

```hcl
# PostgreSQL Database - Computing Resource
module "db_tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  capacity_values = [8]  # 8 vCores
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

resource "azurerm_postgresql_flexible_server" "main" {
  name                   = "psql-dialogporten-prod"
  resource_group_name    = azurerm_resource_group.main.name
  location              = azurerm_resource_group.main.location
  version               = "14"
  administrator_login    = "psqladmin"
  administrator_password = "H@Sh1CoR3!"
  
  sku_name = "GP_Standard_D8s_v3"
  
  tags = module.db_tags.tags  # Includes finops_capacity = "8vcpu"

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}
```

### Non-Computing Resources (without capacity)

Non-computing resources like storage accounts, networking, and key vaults should not include capacity information:

```hcl
# Storage Account - Non-Computing Resource
data "azurerm_client_config" "current" {}

module "storage_tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  # capacity_values = []  # Default - no capacity for storage
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

resource "azurerm_storage_account" "main" {
  name                     = "stdialogportenprod"
  resource_group_name      = azurerm_resource_group.main.name
  location                 = azurerm_resource_group.main.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  
  tags = module.storage_tags.tags  # Does NOT include finops_capacity tag

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}
```

```hcl
# Virtual Network - Non-Computing Resource
module "vnet_tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  # No capacity_values needed for networking
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

resource "azurerm_virtual_network" "main" {
  name                = "vnet-dialogporten-prod"
  address_space       = ["10.0.0.0/16"]
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name

  tags = module.vnet_tags.tags  # Does NOT include finops_capacity tag

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}
```

```hcl
# Key Vault - Non-Computing Resource
module "kv_tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  # Explicitly disable capacity tag for non-computing resource
  include_capacity_tag = false
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

resource "azurerm_key_vault" "main" {
  name                = "kv-dialogporten-prod"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  tenant_id           = data.azurerm_client_config.current.tenant_id
  sku_name           = "standard"

  tags = module.kv_tags.tags  # Does NOT include finops_capacity tag

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}
```

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
  capacity_values = [4, 2]  # webapp: 4 vCPUs, database: 2 vCPUs
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
  capacity_values = [2, 1, 4]  # app_service: 2, functions: 1, containers: 4
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
  pool_capacities = [
    for pool_name, pool in var.pool_configs : 
    pool.max_count * local.vm_size_to_vcpus[lower(pool.vm_size)]
  ]
}

# Get current Azure client configuration
data "azurerm_client_config" "current" {}

module "tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  capacity_values         = local.pool_capacities  # [12, 24]
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
  capacity_breakdown = [
    36,  # aks_cluster - calculated from node pools above
    8,   # app_services - 4 instances × 2 vCPUs each  
    2,   # function_apps - 2 dedicated function apps
    4    # sql_database - DTU converted to approximate vCPU equivalent
  ]
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
# This should only be used for computing resources
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
  capacity_values = [4]  # 4 vCPUs
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
  capacity_values = [4]  # 4 vCPUs
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
  capacity_values = [2]  # dev_resources: 2 vCPUs
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
  capacity_values = [16, 32, 8, 4]  # web_tier: 16, app_tier: 32, data_tier: 8, cache_tier: 4
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
  capacity_values = [4]  # test_load: 4 vCPUs
  repository   = "github.com/altinn/altinn-formidling"
  current_user = "test-automation-sp"
}
```

## Error Handling and Debugging Examples

### Service Owner Code Validation

```hcl
# Example handling invalid service owner code
data "azurerm_client_config" "current" {}

module "tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "invalidcode"  # This will cause validation to fail
  capacity_values = [4]  # 4 vCPUs
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

# This will output debug information to help identify the issue
output "debug_service_owner" {
  value = {
    validation_info = module.tags.service_owner_validation
    your_code       = "invalidcode"
    available_codes = module.tags.available_service_owner_codes
  }
}

# Error message will be:
# Service owner code 'invalidcode' not found in Altinn organization registry.
# Check https://altinncdn.no/orgs/altinn-orgs.json for valid codes or provide finops_serviceownerorgnr manually.
```

### External API Error Handling

```hcl
# Example with manual override when external API is unavailable
data "azurerm_client_config" "current" {}

module "tags" {
  source                   = "./modules/tags"
  finops_environment       = "prod"
  finops_product           = "dialogporten"
  finops_serviceownercode  = "skd"
  finops_serviceownerorgnr = "974761076"  # Manual override bypasses external lookup
  capacity_values = [4]  # 4 vCPUs
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

# This approach works even if altinncdn.no is unavailable
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

### Debug Information Usage

```hcl
# Example showing how to debug service owner code issues
data "azurerm_client_config" "current" {}

module "tags" {
  source                  = "./modules/tags"
  finops_environment      = "dev"
  finops_product          = "studio"
  finops_serviceownercode = "skd"
  capacity_values = [4]  # 4 vCPUs
  repository   = "github.com/altinn/altinn-studio"
  current_user = data.azurerm_client_config.current.client_id
}

# Debug outputs to verify everything is working correctly
output "tags_debug" {
  description = "Debug information for troubleshooting"
  value = {
    generated_tags        = module.tags.tags
    validation_status     = module.tags.service_owner_validation
    organization_name     = module.tags.organization_name
    total_capacity        = module.tags.total_vcpus
    capacity_breakdown    = module.tags.capacity_breakdown
  }
}

output "available_service_owners" {
  description = "List all available service owner codes for reference"
  value       = module.tags.available_service_owner_codes
}

# Example output:
# tags_debug = {
#   generated_tags = {
#     finops_environment = "dev"
#     finops_product = "studio"
#     finops_serviceownercode = "skd"
#     finops_serviceownerorgnr = "974761076"
#     finops_capacity = "4vcpu"
#     createdby = "user@altinn.no"
#     # ... other tags
#   }
#   validation_status = {
#     service_owner_exists = true
#     using_manual_override = false
#     available_codes_count = 150
#     external_data_loaded = true
#   }
#   organization_name = "Skatteetaten"
#   total_capacity = 4
#   capacity_breakdown = [4]
# }
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
  
  # Calculate total capacity from all sources
  total_capacity = [
    3 * local.vm_size_to_vcpus["standard_b4s_v2"],  # system_pool: 12 vCPUs
    5 * local.vm_size_to_vcpus["standard_d4s_v3"],  # user_pool: 20 vCPUs
    8,   # app_services: 4 instances × 2 vCPUs
    2,   # function_apps: 2 consumption plan apps
    4    # sql_databases: DTU equivalent
  ]
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

# Debug outputs for troubleshooting
output "service_owner_debug" {
  description = "Service owner validation and debug information"
  value = {
    validation_info   = module.tags.service_owner_validation
    organization_name = module.tags.organization_name
    using_code       = "skd"
    resolved_orgnr   = module.tags.finops_serviceownerorgnr
  }
}
```

This example demonstrates:
- Capacity calculation from multiple sources (AKS node pools, app services, etc.) for computing resources
- Proper lifecycle rules for immutability
- Real-world resource configuration
- Output values for verification and debugging
- Dynamic environment handling
- Service principal authentication
- Error handling and validation debugging
- Appropriate use of capacity tags only for computing resources (AKS cluster)

## Mixed Computing and Non-Computing Resources Example

This comprehensive example shows how to properly tag both computing and non-computing resources in the same infrastructure:

```hcl
# Get current Azure client configuration
data "azurerm_client_config" "current" {}

# Calculate capacity for computing resources only
locals {
  vm_size_to_vcpus = {
    "standard_b4s_v2" = 4
    "standard_d4s_v3" = 4
  }
  
  # Calculate capacity for different resource types
  aks_capacity = [
    3 * local.vm_size_to_vcpus["standard_b4s_v2"],  # system_pool: 12 vCPUs
    5 * local.vm_size_to_vcpus["standard_d4s_v3"]   # user_pool: 20 vCPUs
  ]
  
  db_capacity = [8, 4]    # primary_db: 8 vCores, cache_db: 4 vCores
  
  app_capacity = [8, 4]   # web_tier: 8 vCPUs, api_tier: 4 vCPUs
}

# Resource Group (non-computing - no capacity)
module "rg_tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  # No capacity_values - resource groups don't compute
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

resource "azurerm_resource_group" "main" {
  name     = "rg-dialogporten-prod"
  location = "Norway East"
  tags     = module.rg_tags.tags  # No finops_capacity tag

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

# AKS Cluster (computing - includes capacity)
module "aks_tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  capacity_values         = local.aks_capacity  # [12, 20] = 32 vCPUs total
  repository              = "github.com/altinn/dialogporten"
  current_user            = data.azurerm_client_config.current.client_id
}

resource "azurerm_kubernetes_cluster" "main" {
  name                = "aks-dialogporten-prod"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  dns_prefix          = "aks-dialogporten-prod"

  default_node_pool {
    name       = "system"
    node_count = 3
    vm_size    = "Standard_B4s_v2"
  }

  identity {
    type = "SystemAssigned"
  }

  tags = module.aks_tags.tags  # Includes finops_capacity = "32vcpu"

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

# PostgreSQL Database (computing - includes capacity)
module "db_tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  capacity_values         = local.db_capacity  # [8, 4] = 12 vCPUs total
  repository              = "github.com/altinn/dialogporten"
  current_user            = data.azurerm_client_config.current.client_id
}

resource "azurerm_postgresql_flexible_server" "main" {
  name                = "psql-dialogporten-prod"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  version             = "14"
  
  sku_name = "GP_Standard_D8s_v3"
  
  tags = module.db_tags.tags  # Includes finops_capacity = "12vcpu"

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

# App Service (computing - includes capacity)
module "app_tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  capacity_values         = local.app_capacity  # [8, 4] = 12 vCPUs total
  repository              = "github.com/altinn/dialogporten"
  current_user            = data.azurerm_client_config.current.client_id
}

resource "azurerm_service_plan" "main" {
  name                = "plan-dialogporten-prod"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  os_type             = "Linux"
  sku_name            = "P1v2"

  tags = module.app_tags.tags  # Includes finops_capacity = "12vcpu"

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

# Storage Account (non-computing - no capacity)
module "storage_tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  # No capacity_values - storage doesn't compute
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

resource "azurerm_storage_account" "main" {
  name                     = "stdialogportenprod"
  resource_group_name      = azurerm_resource_group.main.name
  location                 = azurerm_resource_group.main.location
  account_tier             = "Standard"
  account_replication_type = "LRS"

  tags = module.storage_tags.tags  # No finops_capacity tag

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

# Virtual Network (non-computing - no capacity)
module "vnet_tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  # No capacity_values - networking doesn't compute
  repository   = "github.com/altinn/dialogporten"
  current_user = data.azurerm_client_config.current.client_id
}

resource "azurerm_virtual_network" "main" {
  name                = "vnet-dialogporten-prod"
  address_space       = ["10.0.0.0/16"]
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name

  tags = module.vnet_tags.tags  # No finops_capacity tag

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

# Key Vault (non-computing - explicitly disabled capacity)
module "kv_tags" {
  source                  = "./modules/tags"
  finops_environment      = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  include_capacity_tag    = false  # Explicitly disable for clarity
  repository              = "github.com/altinn/dialogporten"
  current_user            = data.azurerm_client_config.current.client_id
}

resource "azurerm_key_vault" "main" {
  name                = "kv-dialogporten-prod"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  tenant_id           = data.azurerm_client_config.current.tenant_id
  sku_name            = "standard"

  tags = module.kv_tags.tags  # No finops_capacity tag

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}

# Outputs showing different capacity calculations
output "capacity_summary" {
  description = "Summary of capacity allocations across computing resources"
  value = {
    aks_capacity     = module.aks_tags.total_vcpus      # 32
    db_capacity      = module.db_tags.total_vcpus       # 12
    app_capacity     = module.app_tags.total_vcpus      # 12
    total_computing  = module.aks_tags.total_vcpus + module.db_tags.total_vcpus + module.app_tags.total_vcpus  # 56
    
    # Non-computing resources don't have capacity
    storage_capacity = module.storage_tags.total_vcpus  # 0
    vnet_capacity    = module.vnet_tags.total_vcpus     # 0
    kv_capacity      = module.kv_tags.total_vcpus       # 0
  }
}

output "resource_types" {
  description = "Which resources are tagged as computing resources"
  value = {
    aks_computing     = module.aks_tags.is_computing_resource     # true
    db_computing      = module.db_tags.is_computing_resource      # true
    app_computing     = module.app_tags.is_computing_resource     # true
    storage_computing = module.storage_tags.is_computing_resource # false
    vnet_computing    = module.vnet_tags.is_computing_resource    # false
    kv_computing      = module.kv_tags.is_computing_resource      # false
  }
}
```

**This example demonstrates proper FinOps tagging:**

**Computing Resources (with finops_capacity tag):**
- AKS Cluster: `finops_capacity = "32vcpu"`
- PostgreSQL Database: `finops_capacity = "12vcpu"`  
- App Service Plan: `finops_capacity = "12vcpu"`

**Non-Computing Resources (without finops_capacity tag):**
- Resource Group: No capacity tag
- Storage Account: No capacity tag
- Virtual Network: No capacity tag
- Key Vault: No capacity tag (explicitly disabled)

**Benefits of this approach:**
- Accurate cost allocation for computing vs. storage/networking costs
- Clear separation of capacity-based and non-capacity-based resources
- Simplified FinOps reporting and analysis
- Compliance with capacity tagging best practices