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
    http = {
      source  = "hashicorp/http"
      version = "~> 3.4"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.9"
    }
  }
}
```

### 3. Set Variables

Create or update your `terraform.tfvars`:

```hcl
finops_environment      = "prod"
finops_product          = "dialogporten"
finops_serviceownercode = "skd"
repository              = "github.com/altinn/dialogporten"
current_user            = "terraform-sp"
created_date            = "2024-03-15"
modified_date           = ""
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

All resources receive the same standardized tags:
```hcl
finops_environment       = "prod"
finops_product           = "dialogporten"
finops_serviceownercode  = "skd"
finops_serviceownerorgnr = "974761076"
createdby                = "terraform-sp"
createddate              = "2024-03-15"
modifiedby               = "terraform-sp"
modifieddate             = "2024-03-15"
repository               = "github.com/altinn/dialogporten"
```

## Configuration

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `finops_environment` | Environment name | `"prod"` |
| `finops_product` | Product name | `"dialogporten"` |
| `finops_serviceownercode` | Service owner code | `"skd"` |
| `repository` | Repository URL | `"github.com/altinn/dialogporten"` |
| `current_user` | User/service principal | `"terraform-sp"` |

### Optional Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `created_date` | current date | Creation date (YYYY-MM-DD) |
| `modified_date` | current date | Modification date (YYYY-MM-DD) |

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

### Prevent Tag Drift

```hcl
resource "azurerm_resource_group" "main" {
  tags = local.base_tags

  lifecycle {
    ignore_changes = [tags["createdby"], tags["createddate"]]
  }
}
```

## CI/CD Integration

### Azure DevOps

```yaml
  variables:
    TF_VAR_current_user: "$(Build.RequestedFor)"
    TF_VAR_modified_date: "$[format('{0:yyyy-MM-dd}', pipeline.startTime)]"

steps:
  - task: TerraformTaskV4@4
    inputs:
      command: 'apply'
      commandOptions: '-var="finops_environment=$(Environment)"'
```

### GitHub Actions

```yaml
- name: Set Terraform Variables
  run: |
    echo "TF_VAR_current_user=${{ github.actor }}" >> $GITHUB_ENV
    echo "TF_VAR_modified_date=$(date +%Y-%m-%d)" >> $GITHUB_ENV

- name: Terraform Apply
  run: terraform apply -auto-approve
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

### Creation Date Changes Every Plan

**Error:**
```
  ~ tags = {
      ~ createddate = "2024-10-24" -> "2024-10-25"
    }
```

**Solution:**
This is normal behavior for the first few plans. The `time_static` resource ensures creation_date stabilizes after the initial deployment and won't change on subsequent plans.

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
