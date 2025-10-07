# Tags Terraform Module

This Terraform module provides standardized tags for Azure resources following Altinn FinOps requirements (Fase 1: Kostnadsfordeling). It creates a consistent set of tags that can be applied across all resources in your infrastructure to improve cost tracking, governance, and resource management.

## Features

- Standardized FinOps tags for cost allocation and tracking
- Automatic creation and modification timestamps
- Lowercase normalization of key values
- Repository tracking for infrastructure as code traceability

## Usage

### Basic Example

```hcl
module "tags" {
  source                      = "./modules/tags"
  finops_environment          = "prod"
  finops_product              = "dialogporten"
  finops_serviceownercode     = "skd"
  finops_serviceownerorgnr    = "974761076"
  capacity_values = {
    webapp      = 4
    database    = 2
  }
  repository                  = "github.com/altinn/dialogporten"
}

resource "azurerm_storage_account" "example" {
  name                     = "mystorageaccount"
  resource_group_name      = azurerm_resource_group.rg.name
  location                 = azurerm_resource_group.rg.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  tags                     = module.tags.tags
}
```

### Complete Example with Resource Group

```hcl
module "tags" {
  source                      = "./modules/tags"
  finops_environment          = "dev"
  finops_product              = "studio"
  finops_serviceownercode     = "skd"
  finops_serviceownerorgnr    = "974761076"
  capacity_values = {
    app_service = 2
    functions   = 1
    containers  = 4
  }
  repository                  = "github.com/altinn/altinn-studio"
}

resource "azurerm_resource_group" "main" {
  name     = "rg-studio-dev"
  location = "Norway East"
  tags     = module.tags.tags
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
}
```

### Calculating Capacity from Node Pools

For AKS clusters or other scenarios where you need to calculate capacity from multiple pools:

```hcl
# Define your pool configurations
variable "pool_configs" {
  default = {
    syspool = {
      vm_size    = "standard_b4s_v2"
      max_count  = 3
    }
    workpool = {
      vm_size    = "standard_b4s_v2" 
      max_count  = 6
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

module "tags" {
  source                   = "./modules/tags"
  finops_environment       = "prod"
  finops_product          = "dialogporten"
  finops_serviceownercode = "skd"
  finops_serviceownerorgnr = "974761076"
  capacity_values         = local.pool_capacities  # { syspool = 12, workpool = 24 }
  repository              = "github.com/altinn/dialogporten"
}

# Result: finops_capacity tag = "36vcpu" (12 + 24)
```

### Mixed Resource Capacity Calculation

For environments with different types of resources:

```hcl
locals {
  # Calculate capacity from different resource types
  capacity_breakdown = {
    aks_cluster    = 36  # Calculated from node pools above
    app_services   = 8   # 4 instances × 2 vCPUs each
    function_apps  = 2   # 2 dedicated function apps
    sql_database   = 4   # DTU converted to approximate vCPU equivalent
  }
}

module "tags" {
  source                   = "./modules/tags"
  finops_environment       = "prod"
  finops_product          = "studio"
  finops_serviceownercode = "skd"
  finops_serviceownerorgnr = "974761076"
  capacity_values         = local.capacity_breakdown
  repository              = "github.com/altinn/altinn-studio"
}

# Result: finops_capacity tag = "50vcpu" (36 + 8 + 2 + 4)
```

## Variables

| Name | Description | Type | Required | Validation | Example |
|------|-------------|------|----------|------------|---------|
| `finops_environment` | Environment designation for cost allocation | `string` | Yes | Must be one of: `dev`, `test`, `prod`, `at22`, `at23`, `at24`, `yt01`, `tt02` | `"prod"` |
| `finops_product` | Product name for cost allocation | `string` | Yes | Must be one of: `studio`, `dialogporten`, `formidling`, `autorisasjon`, `varsling`, `melding`, `altinn2` | `"dialogporten"` |
| `finops_serviceownercode` | Service owner code for billing attribution | `string` | Yes | Must be `skd`, `udir`, `nav`, `na`, or 2-5 lowercase letters | `"skd"` |
| `finops_serviceownerorgnr` | Service owner organization number (9 digits) | `string` | Yes | Exactly 9 digits | `"974761076"` |
| `capacity_values` | Map of capacity values (in vCPUs) to be summed for total finops_capacity | `map(number)` | No (default: {}) | All values must be non-negative numbers | `{ syspool = 12, workpool = 24 }` |
| `repository` | Repository URL for infrastructure traceability | `string` | Yes | Must be from `github.com/altinn/` organization | `"github.com/altinn/dialogporten"` |
| `createdby` | Who or what created the resource | `string` | No (default: "terraform") | Must be `terraform`, `azure-policy`, or valid username | `"terraform"` |
| `modifiedby` | Who or what last modified the resource | `string` | No (default: "terraform") | Must be `terraform`, `azure-policy`, or valid username | `"terraform"` |

## Outputs

| Name | Description | Type |
|------|-------------|------|
| `tags` | Map of all standardized tags | `map(string)` |
| `finops_environment` | Normalized environment name | `string` |
| `finops_product` | Normalized product name | `string` |
| `finops_serviceownercode` | Normalized service owner code | `string` |
| `finops_serviceownerorgnr` | Service owner organization number | `string` |
| `finops_capacity` | Total vCPU capacity calculated from provided capacity values | `string` |
| `total_vcpus` | Total vCPU capacity calculated from all provided capacity values | `number` |
| `capacity_breakdown` | Breakdown of individual capacity values used in calculation | `map(number)` |
| `repository` | Normalized repository URL | `string` |
| `createdby` | Who or what created the resource | `string` |
| `modifiedby` | Who or what last modified the resource | `string` |
| `created_date` | Date when the tags were created | `string` |
| `modified_date` | Date when the tags were last modified | `string` |

## Generated Tags

The module automatically generates the following tags according to Altinn FinOps requirements:

### FinOps Tags (5 tags with "finops_" prefix)
| Tag Name | Description | Example Value |
|----------|-------------|---------------|
| `finops_environment` | Environment for cost separation | `"prod"` |
| `finops_product` | Main product allocation for cost distribution | `"dialogporten"` |
| `finops_serviceownercode` | Service owner code for billing | `"skd"` |
| `finops_serviceownerorgnr` | Formal service owner identification | `"974761076"` |
| `finops_capacity` | Capacity planning and cost optimization | `"36vcpu"` |

### Traceability Tags (5 tags)
| Tag Name | Description | Example Value |
|----------|-------------|---------------|
| `createdby` | Who/what created the resource | `"terraform"` |
| `createddate` | Resource creation date (YYYY-MM-DD) | `"2024-01-15"` |
| `modifiedby` | Who/what last modified the resource | `"terraform"` |
| `modifieddate` | Last modification date (YYYY-MM-DD) | `"2024-01-15"` |
| `repository` | IaC repository for traceability | `"github.com/altinn/dialogporten"` |

## Best Practices

1. **Consistent Application**: Apply the same tags module across all your Azure resources for consistent cost tracking and governance.

2. **Environment Naming**: Use only the approved environment values: `dev`, `test`, `prod`, `at22`, `at23`, `at24`, `yt01`, `tt02`.

3. **Product Names**: Use only approved product names: `studio`, `dialogporten`, `formidling`, `autorisasjon`, `varsling`, `melding`, `altinn2`.

5. **Capacity Calculation**: Provide capacity values as numbers in the `capacity_values` map. The module will sum them and format as `{total}vcpu`.

5. **Service Owner Codes**: Use approved codes (`skd`, `udir`, `nav`, `na`) or follow the 2-5 lowercase letter pattern.

6. **Repository URLs**: Always use repositories from `github.com/altinn/` organization for traceability.

7. **Lowercase Convention**: All tag names are lowercase and singular, values are normalized to lowercase where appropriate.

## FinOps Integration

These tags are designed to support FinOps practices by providing:

- **Cost Allocation**: Tags enable accurate cost allocation across products, environments, and teams
- **Resource Governance**: Consistent tagging helps with resource lifecycle management
- **Compliance**: Standardized tags support compliance and audit requirements
- **Automation**: Tags can be used for automated resource management and policies

## Module Structure

The module is organized into the following files:

- `variables.tf` - Input variable definitions with validation rules
- `locals.tf` - Tag computation and normalization logic
- `outputs.tf` - Output definitions for consuming modules
- `versions.tf` - Terraform and provider version constraints
- `tags.tf` - Main module documentation and organization
- `README.md` - This documentation file

## Validation Rules

The module includes built-in validation to ensure compliance with Altinn FinOps requirements:

- **Environment**: Must be exactly one of `dev`, `test`, `prod`, `at22`, `at23`, `at24`, `yt01`, `tt02`
- **Product**: Must be exactly one of `studio`, `dialogporten`, `formidling`, `autorisasjon`, `varsling`, `melding`, `altinn2`
- **Service Owner Code**: Must be `skd`, `udir`, `nav`, `na`, or 2-5 lowercase letters
- **Organization Number**: Must be exactly 9 digits
- **Capacity Values**: All values in the map must be non-negative numbers
- **Repository**: Must be from `github.com/altinn/` organization
- **Created/Modified By**: Must be `terraform`, `azure-policy`, or valid username format

## Requirements

- Terraform >= 1.0
- Time Provider ~> 0.9

## License

This module is maintained as part of the Altinn platform infrastructure.