# Tags Terraform Module

This Terraform module provides standardized tags for Azure resources following Altinn FinOps requirements (Fase 1: Kostnadsfordeling). It creates a consistent set of tags that can be applied across all resources in your infrastructure to improve cost tracking, governance, and resource management.

## Features

- Standardized FinOps tags for cost allocation and tracking
- Automatic organization number lookup from Altinn CDN
- Flexible capacity calculation from multiple input sources
- Automatic creation and modification timestamps
- Lowercase normalization of key values
- Repository tracking for infrastructure as code traceability

## Quick Start

```hcl
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
  name = "mystorageaccount"
  # ... other configuration
  tags = module.tags.tags

  lifecycle {
    ignore_changes = [
      tags["createdby"],
      tags["createddate"]
    ]
  }
}
```

For detailed examples, see [EXAMPLES.md](./EXAMPLES.md).

## Variables

| Name | Description | Type | Required | Default | Validation | Example |
|------|-------------|------|----------|---------|------------|---------|
| `capacity_values` | List of capacity values (in vCPUs) to be summed for total finops_capacity. Only provide for computing resources. | `list(number)` | No | `[]` | All values must be non-negative numbers | `[12, 24]` |
| `include_capacity_tag` | Whether to include finops_capacity tag. For computing resources only (AKS, VMs, PostgreSQL, App Services). | `bool` | No | Auto-determined from capacity_values | Must be true, false, or null | `true` |
| `current_user` | Current user/service principal running Terraform | `string` | Yes | - | Must be meaningful identity with at least 3 characters | `"john.doe@altinn.no"` |
| `finops_environment` | Environment designation for cost allocation | `string` | Yes | - | Must be one of: `dev`, `test`, `prod`, `at22`, `at23`, `at24`, `yt01`, `tt02` | `"prod"` |
| `finops_product` | Product name for cost allocation | `string` | Yes | - | Must be one of: `studio`, `dialogporten`, `formidling`, `autorisasjon`, `varsling`, `melding`, `altinn2` | `"dialogporten"` |
| `finops_serviceownercode` | Service owner code for billing attribution | `string` | Yes | - | Must be letters only | `"skd"` |
| `finops_serviceownerorgnr` | Service owner organization number override (optional) | `string` | No | `null` | Exactly 9 digits when provided | `"974761076"` |
| `repository` | Repository URL for infrastructure traceability | `string` | Yes | - | Must be from `github.com/altinn/` organization | `"github.com/altinn/dialogporten"` |

## Outputs

| Name | Description | Type |
|------|-------------|------|
| `tags` | Map of all standardized tags | `map(string)` |
| `finops_environment` | Normalized environment name | `string` |
| `finops_product` | Normalized product name | `string` |
| `finops_serviceownercode` | Normalized service owner code | `string` |
| `finops_serviceownerorgnr` | Service owner organization number (provided as input or automatically looked up) | `string` |
| `finops_capacity` | Total vCPU capacity calculated from provided capacity values | `string` |
| `total_vcpus` | Total vCPU capacity calculated from all provided capacity values | `number` |
| `capacity_breakdown` | List of individual capacity values used in calculation | `list(number)` |
| `repository` | Normalized repository URL | `string` |
| `createdby` | Who or what created the resource (set to current_user) | `string` |
| `modifiedby` | Who or what last modified the resource (set to current_user) | `string` |
| `created_date` | Date when the tags were created (set to today) | `string` |
| `modified_date` | Date when the tags were last modified (set to today) | `string` |
| `organization_name` | Organization name in Norwegian (only available when using automatic lookup) | `string` |
| `service_owner_validation` | Debug information for service owner code validation | `object` |
| `available_service_owner_codes` | List of available service owner codes from Altinn CDN (for debugging) | `list(string)` |
| `is_computing_resource` | Whether this resource is tagged as a computing resource with capacity information | `bool` |

## Generated Tags

The module automatically generates the following tags according to Altinn FinOps requirements:

### FinOps Tags (4-5 tags with "finops_" prefix)
| Tag Name | Description | Example Value | Applied To |
|----------|-------------|---------------|------------|
| `finops_environment` | Environment for cost separation | `"prod"` | All resources |
| `finops_product` | Main product allocation for cost distribution | `"dialogporten"` | All resources |
| `finops_serviceownercode` | Service owner code for billing | `"skd"` | All resources |
| `finops_serviceownerorgnr` | Formal service owner identification | `"974761076"` | All resources |
| `finops_capacity` | Capacity planning and cost optimization | `"36vcpu"` | **Computing resources only** |

**Note**: The `finops_capacity` tag is only applied to computing resources such as:
- Azure Kubernetes Service (AKS) clusters
- Virtual Machines and VM Scale Sets  
- PostgreSQL/MySQL databases
- App Services and Function Apps
- Container Instances

It is **not** applied to non-computing resources such as:
- Storage Accounts
- Key Vaults  
- Virtual Networks
- DNS Zones
- Application Insights

### Traceability Tags (5 tags)
| Tag Name | Description | Example Value |
|----------|-------------|---------------|
| `createdby` | Who/what created the resource | `"john.doe@altinn.no"` |
| `createddate` | Resource creation date (YYYY-MM-DD) | `"2024-01-15"` |
| `modifiedby` | Who/what last modified the resource | `"jane.smith@altinn.no"` |
| `modifieddate` | Last modification date (YYYY-MM-DD) | `"2024-01-15"` |
| `repository` | IaC repository for traceability | `"github.com/altinn/dialogporten"` |

## Key Features

### Automatic Organization Lookup with Error Handling

The module automatically fetches organization data from Altinn's CDN and looks up the organization number (`finops_serviceownerorgnr`) based on the provided service owner code. This ensures:

- **Data Consistency**: Organization numbers are always accurate and up-to-date
- **Simplified Usage**: You only need to provide the service owner code
- **Validation**: The module validates that the service owner code exists in the registry
- **Override Option**: You can still manually provide an organization number if needed
- **Resilient**: Includes retry logic and timeout handling for external API calls
- **Error Handling**: Graceful degradation when external data is unavailable

The module includes robust error handling for external API calls:
- 30-second timeout for HTTP requests
- Automatic retry with exponential backoff (3 attempts)
- Validation that service owner codes exist in the fetched data
- Clear error messages when codes are not found

Valid service owner codes include: `skd`, `udir`, `nav`, `digdir`, `brg`, `ssb`, and many others. See the [Altinn organization registry](https://altinncdn.no/orgs/altinn-orgs.json) for the complete list.

### Flexible Capacity Calculation (Computing Resources Only)

The module accepts capacity values as numbers in a `capacity_values` map and automatically sums them to create the `finops_capacity` tag. **This tag is only applied to computing resources** such as AKS clusters, VMs, databases, and app services.

**For Computing Resources:**
```hcl
module "tags" {
  source = "./modules/tags"
  # ... other variables
  capacity_values = [32, 4]  # AKS nodes: 32 vCPUs, Database: 4 vCPUs
  # finops_capacity tag will be "36vcpu"
}
```

**For Non-Computing Resources:**
```hcl
module "tags" {
  source = "./modules/tags"
  # ... other variables
  # capacity_values = [] (default - no capacity tag generated)
  # OR explicitly disable:
  # include_capacity_tag = false
}
```

This approach allows you to:
- Calculate capacity from multiple sources (node pools, app services, databases)
- Keep business logic (VM size mappings, etc.) in your calling code
- Provide transparency through the `capacity_breakdown` output
- Apply capacity tags only where they make sense from a FinOps perspective
- Simple list format without needing descriptive keys

**Before (complex map approach):**
```hcl
capacity_values = {
  system_pool_nodes = 12
  user_pool_nodes   = 20
  app_service_tier  = 8
  database_cores    = 4
}
```

**After (simple list approach):**
```hcl
capacity_values = [12, 20, 8, 4]  # Same total, much simpler!
```

### createdby Immutability Protection

Since the module cannot automatically detect existing resources, immutability protection for `createdby` is handled at the resource level using Terraform lifecycle rules:

```hcl
resource "azurerm_storage_account" "example" {
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

This ensures that:
- `createdby` and `createddate` are set on first creation but never updated
- `modifiedby` and `modifieddate` are updated on each subsequent run
- Original creator information is preserved for audit trails

## Best Practices

1. **Consistent Application**: Apply the same tags module across all your Azure resources for consistent cost tracking and governance.

2. **Capacity Tags for Computing Resources Only**: Only provide `capacity_values` for computing resources (AKS, VMs, PostgreSQL, App Services). For storage accounts, networking, and other non-computing resources, omit capacity values.

3. **Environment Naming**: Use only the approved environment values: `dev`, `test`, `prod`, `at22`, `at23`, `at24`, `yt01`, `tt02`.

4. **Product Names**: Use only approved product names: `studio`, `dialogporten`, `formidling`, `autorisasjon`, `varsling`, `melding`, `altinn2`.

5. **Service Owner Codes**: Use codes that exist in the Altinn organization registry. The organization number will be automatically looked up.

6. **Repository URLs**: Always use repositories from `github.com/altinn/` organization for traceability.

7. **Lifecycle Rules**: Apply lifecycle rules to protect `createdby` and `createddate` from being modified after initial creation.

8. **Meaningful Identities**: Use specific user names, service principal names, or application names for `current_user` rather than generic terms.

### Resource Type Examples

**Computing Resources (include capacity):**
```hcl
# AKS Cluster
module "aks_tags" {
  source = "./modules/tags"
  capacity_values = [32]  # 32 vCPUs total
  # ... other variables
}

# PostgreSQL Database  
module "db_tags" {
  source = "./modules/tags"
  capacity_values = [8]  # 8 vCPUs
  # ... other variables
}
```

**Non-Computing Resources (no capacity):**
```hcl
# Storage Account
module "storage_tags" {
  source = "./modules/tags"
  # capacity_values = [] (default - no capacity tag)
  # ... other variables
}

# Virtual Network
module "vnet_tags" {
  source = "./modules/tags"
  # No capacity values needed
  # ... other variables
}
```

## FinOps Integration

These tags are designed to support FinOps practices by providing:

- **Cost Allocation**: Tags enable accurate cost allocation across products, environments, and teams
- **Resource Governance**: Consistent tagging helps with resource lifecycle management
- **Compliance**: Standardized tags support compliance and audit requirements
- **Automation**: Tags can be used for automated resource management and policies

## Validation Rules

The module includes built-in validation to ensure compliance with Altinn FinOps requirements:

- **Environment**: Must be exactly one of `dev`, `test`, `prod`, `at22`, `at23`, `at24`, `yt01`, `tt02`
- **Product**: Must be exactly one of `studio`, `dialogporten`, `formidling`, `autorisasjon`, `varsling`, `melding`, `altinn2`
- **Service Owner Code**: Must be letters only (case-insensitive, automatically normalized to lowercase)
- **Service Owner Code Existence**: Must exist in the Altinn organization registry (validated against external data)
- **Organization Number Override**: Must be exactly 9 digits when provided
- **Capacity Values**: All values in the map must be non-negative numbers
- **Repository**: Must be from `github.com/altinn/` organization
- **Current User**: Must be at least 3 characters and contain only alphanumeric characters, dots, underscores, @ signs, and hyphens

### Error Handling and Debugging

If you encounter issues with service owner code validation, use the debug outputs:

```hcl
# Check validation status and available codes
output "debug_service_owner" {
  value = {
    validation_info = module.tags.service_owner_validation
    available_codes = module.tags.available_service_owner_codes
  }
}
```

Common error scenarios:
- **Service owner code not found**: The code doesn't exist in the Altinn registry
- **External API unavailable**: Network issues preventing data fetch (gracefully handled)
- **Invalid JSON response**: Malformed data from external API (gracefully handled)

## Requirements

- Terraform >= 1.0
- HTTP Provider ~> 3.4 (for fetching organization data with retry support)
- Network access to `https://altinncdn.no/orgs/altinn-orgs.json`

## Module Structure

The module is organized into the following files:

- `variables.tf` - Input variable definitions with validation rules
- `data.tf` - HTTP data source for fetching organization data
- `locals.tf` - Tag computation and normalization logic
- `outputs.tf` - Output definitions for consuming modules
- `versions.tf` - Terraform and provider version constraints
- `README.md` - This documentation file
- `EXAMPLES.md` - Comprehensive usage examples

## License

This module is maintained as part of the Altinn platform infrastructure.