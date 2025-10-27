# Altinn Platform - Standardized Resource Tagging

A single-file solution for consistent resource tagging across all Terraform projects.

## Quick Start

### 1. Copy the File

Copy `tags.tf` into your Terraform project:

```bash
cp tags.tf /path/to/your/terraform/project/
```

### 2. Add Required Providers

Add to your `providers.tf`:

```hcl
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
    time = {
      source  = "hashicorp/time"
      version = "~> 0.9"
    }
    terraform = {
      source  = "hashicorp/terraform"
      version = "~> 1.0"
    }
  }
}

provider "azurerm" {
  features {}
}
```

### 3. Set Variables

Create or update your `terraform.tfvars`:

#### Minimal Example (required variables only):
```hcl
finops_environment = "prod"
finops_product     = "dialogporten"
```

#### Full Example (all variables):
```hcl
finops_environment       = "prod"
finops_product           = "dialogporten"
finops_serviceownercode  = "skd"          # Optional
finops_serviceownerorgnr = "974761076"    # Optional - overrides automatic lookup
repository               = "github.com/altinn/dialogporten"  # Optional
```

### 4. Tag Your Resources

```hcl
# Option 1 - Use base tags directly
resource "azurerm_kubernetes_cluster" "main" {
  name = "aks-example"
  # ... configuration
  tags = local.base_tags
}

# Option 2 - Add custom tags with merge()
resource "azurerm_storage_account" "main" {
  name = "stexample"
  # ... configuration
  tags = merge(local.base_tags, {
    providedby = "teamname"
  })
}
```

## Generated Tags

All resources receive the same standardized tags (when all variables are provided):
```hcl
finops_environment       = "prod"
finops_product           = "dialogporten"
finops_serviceownercode  = "skd"                    # Empty if not provided
finops_serviceownerorgnr = "974761076"              # Manual override or automatic lookup
createdby                = "terraform-sp"
createddate              = "2024-10-24"             # Current date when created (stable via time_static)
modifiedby               = "terraform-sp"
modifieddate             = "2024-10-24"             # Current date per run if not provided (will drift)
repository               = "github.com/altinn/dialogporten"  # Empty if not provided
```

## Configuration

### Required Variables

| Variable             | Description              | Example |
|----------------------|--------------------------|---------|
| `finops_environment` | Environment name         | `"prod"` |
| `finops_product`     | Product name             | `"dialogporten"` |

### Optional Variables

| Variable                    | Default | Description |
|----------------------------|---------|-------------|
| `finops_serviceownercode`  | `""`    | Service owner code for billing attribution |
| `finops_serviceownerorgnr` | `""`    | Organization number for billing attribution (overrides automatic lookup) |
| `repository`               | `""`    | Repository URL for infrastructure traceability |



### Valid Values

**Environments:**
`dev`, `test`, `prod`, `at22`, `at23`, `at24`, `yt01`, `tt02`

**Products:**
`studio`, `dialogporten`, `formidling`, `autorisasjon`, `varsling`, `melding`, `altinn2`

**Service Owner Codes:**
Check [https://altinncdn.no/orgs/altinn-orgs.json](https://altinncdn.no/orgs/altinn-orgs.json)

## Best Practices

### Adding Custom Tags

Use the `merge()` function to add custom tags while preserving base tags:

```hcl
resource "azurerm_resource_group" "main" {
  tags = merge(local.base_tags, {
    providedby = "teamname"
    managed    = "terraform"
  })
}
```





## Troubleshooting

### Service Owner Code Not Found

**Error:**
```hcl
 Service owner code 'xyz' not found in Altinn organization registry.
```

**Solution:**
Use a valid code from [https://altinncdn.no/orgs/altinn-orgs.json](https://altinncdn.no/orgs/altinn-orgs.json)

### HTTP Timeout

**Error:**
```hcl
 Error retrieving data from https://altinncdn.no/orgs/altinn-orgs.json
```

**Solution:**
The external API is down. This is rare but can happen. The validation will prevent deployment until the API is available again.



## Files

- `tags.tf` - Main implementation (copy this into your project)
- `tags-example.tf` - Complete usage examples
- `README.md` - This documentation

## Available Tag Sets

- `local.base_tags` - Standard tags for all resources
- `merge(local.base_tags, {...})` - Base tags combined with custom tags

## Benefits

- ✅ **Simple**: One file, copy & paste
- ✅ **Reliable**: Minimal external dependencies
- ✅ **Consistent**: Standardized tags across all projects
- ✅ **Flexible**: Easy to add custom tags with merge()
- ✅ **FinOps compliant**: Follows Altinn tagging requirements

---

**Ready to use!** Copy `tags.tf` into your project and start tagging consistently.
