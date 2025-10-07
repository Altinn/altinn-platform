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
  capacity_values = {
    webapp   = 4
    database = 2
  }
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
| `capacity_values` | Map of capacity values (in vCPUs) to be summed for total finops_capacity | `map(number)` | No | `{}` | All values must be non-negative numbers | `{ syspool = 12, workpool = 24 }` |
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
| `capacity_breakdown` | Breakdown of individual capacity values used in calculation | `map(number)` |
| `repository` | Normalized repository URL | `string` |
| `createdby` | Who or what created the resource (set to current_user) | `string` |
| `modifiedby` | Who or what last modified the resource (set to current_user) | `string` |
| `created_date` | Date when the tags were created (set to today) | `string` |
| `modified_date` | Date when the tags were last modified (set to today) | `string` |
| `organization_name` | Organization name in Norwegian (only available when using automatic lookup) | `string` |

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
| `createdby` | Who/what created the resource | `"john.doe@altinn.no"` |
| `createddate` | Resource creation date (YYYY-MM-DD) | `"2024-01-15"` |
| `modifiedby` | Who/what last modified the resource | `"jane.smith@altinn.no"` |
| `modifieddate` | Last modification date (YYYY-MM-DD) | `"2024-01-15"` |
| `repository` | IaC repository for traceability | `"github.com/altinn/dialogporten"` |

## Key Features

### Automatic Organization Lookup

The module automatically fetches organization data from Altinn's CDN and looks up the organization number (`finops_serviceownerorgnr`) based on the provided service owner code. This ensures:

- **Data Consistency**: Organization numbers are always accurate and up-to-date
- **Simplified Usage**: You only need to provide the service owner code
- **Validation**: The module validates that the service owner code exists in the registry
- **Override Option**: You can still manually provide an organization number if needed

Valid service owner codes include: `skd`, `udir`, `nav`, `digdir`, `brg`, `ssb`, and many others. See the [Altinn organization registry](https://altinncdn.no/orgs/altinn-orgs.json) for the complete list.

### Flexible Capacity Calculation

The module accepts capacity values as numbers in a `capacity_values` map and automatically sums them to create the `finops_capacity` tag. This allows you to:

- Calculate capacity from multiple sources (node pools, app services, databases)
- Keep business logic (VM size mappings, etc.) in your calling code
- Provide transparency through the `capacity_breakdown` output

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

2. **Environment Naming**: Use only the approved environment values: `dev`, `test`, `prod`, `at22`, `at23`, `at24`, `yt01`, `tt02`.

3. **Product Names**: Use only approved product names: `studio`, `dialogporten`, `formidling`, `autorisasjon`, `varsling`, `melding`, `altinn2`.

4. **Service Owner Codes**: Use codes that exist in the Altinn organization registry. The organization number will be automatically looked up.

5. **Repository URLs**: Always use repositories from `github.com/altinn/` organization for traceability.

6. **Lifecycle Rules**: Apply lifecycle rules to protect `createdby` and `createddate` from being modified after initial creation.

7. **Meaningful Identities**: Use specific user names, service principal names, or application names for `current_user` rather than generic terms.

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
- **Organization Number Override**: Must be exactly 9 digits when provided
- **Capacity Values**: All values in the map must be non-negative numbers
- **Repository**: Must be from `github.com/altinn/` organization
- **Current User**: Must be at least 3 characters and contain only alphanumeric characters, dots, underscores, @ signs, and hyphens

## Requirements

- Terraform >= 1.0
- HTTP Provider ~> 3.4 (for fetching organization data)

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