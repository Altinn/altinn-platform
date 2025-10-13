# Altinn Platform - Standardized Resource Tagging

A single-file solution for consistent resource tagging across all Terraform projects.

## Quick Start

### 1. Copy the File

Copy `tags.tf` into your Terraform project:

```bash
cp tags.tf /path/to/your/terraform/project/
```

### 2. Add Required Provider

Add to your `versions.tf`:

```hcl
terraform {
  required_providers {
    http = {
      source  = "hashicorp/http"
      version = "~> 3.4"
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
capacity_values         = [32, 8, 4]
repository              = "github.com/altinn/dialogporten"
current_user            = "terraform-sp"
created_date            = "2024-03-15"
modified_date           = ""
```

### 4. Tag Your Resources

```hcl
# Computing resources (includes capacity tag)
resource "azurerm_kubernetes_cluster" "main" {
  name = "aks-example"
  # ... configuration
  tags = local.base_tags_with_capacity
}

# Non-computing resources (no capacity tag)
resource "azurerm_storage_account" "main" {
  name = "stexample"
  # ... configuration
  tags = local.base_tags
}
```

## Resource Types

### Computing Resources → `local.base_tags_with_capacity`
- Azure Kubernetes Service (AKS)
- Virtual Machines
- PostgreSQL / MySQL Flexible Servers
- App Service Plans
- Azure Container Instances

### Non-Computing Resources → `local.base_tags`
- Storage Accounts
- Key Vaults
- Virtual Networks
- Resource Groups
- Application Insights

## Generated Tags

### Computing Resources
```
finops_environment       = "prod"
finops_product           = "dialogporten"
finops_serviceownercode  = "skd"
finops_serviceownerorgnr = "974761076"
finops_capacity          = "44vcpu"          ← Only on computing resources
createdby                = "terraform-sp"
createddate              = "2024-03-15"
modifiedby               = "terraform-sp"
modifieddate             = "2024-03-15"
repository               = "github.com/altinn/dialogporten"
```

### Non-Computing Resources
Same as above, but without `finops_capacity`.

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
| `capacity_values` | `[]` | vCPU values for capacity calculation |
| `created_date` | current date | Creation date (YYYY-MM-DD) |
| `modified_date` | current date | Modification date (YYYY-MM-DD) |

### Valid Values

**Environments:**
`dev`, `test`, `prod`, `at22`, `at23`, `at24`, `yt01`, `tt02`

**Products:**
`studio`, `dialogporten`, `formidling`, `autorisasjon`, `varsling`, `melding`, `altinn2`

**Service Owner Codes:**
Check https://altinncdn.no/orgs/altinn-orgs.json

## Best Practices

### Prevent Tag Drift

```hcl
resource "azurerm_resource_group" "main" {
  tags = local.base_tags
  
  lifecycle {
    ignore_changes = [tags["createdby"], tags["createddate"]]
  }
}
```

### Resource-Specific Capacity

If you need different capacity per resource:

```hcl
locals {
  aks_capacity = 32
  db_capacity  = 8
  
  aks_tags = merge(local.base_tags, { finops_capacity = "${local.aks_capacity}vcpu" })
  db_tags  = merge(local.base_tags, { finops_capacity = "${local.db_capacity}vcpu" })
}

resource "azurerm_kubernetes_cluster" "main" {
  tags = local.aks_tags  # finops_capacity = "32vcpu"
}

resource "azurerm_postgresql_flexible_server" "main" {
  tags = local.db_tags   # finops_capacity = "8vcpu"
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
```
Service owner code 'xyz' not found in Altinn organization registry.
```

**Solution:**
Use a valid code from https://altinncdn.no/orgs/altinn-orgs.json

### HTTP Timeout

**Error:**
```
Error retrieving data from https://altinncdn.no/orgs/altinn-orgs.json
```

**Solution:**
The external API is down. This is rare but can happen. The validation will prevent deployment until the API is available again.

## Files

## Available Tag Sets

- `local.base_tags` - Standard tags for non-computing resources (no capacity)
- `local.base_tags_with_capacity` - Tags for computing resources (includes finops_capacity)

## Files

- `tags.tf` - Main implementation (copy this into your project)
- `tags-example.tf` - Complete usage examples
- `TAGS-README.md` - This documentation

## Benefits

- ✅ **Simple**: One file, copy & paste
- ✅ **Fast**: No module resolution overhead
- ✅ **Reliable**: Minimal external dependencies
- ✅ **Consistent**: Standardized tags across all projects
- ✅ **FinOps compliant**: Follows Altinn tagging requirements

---

**Ready to use!** Copy `tags.tf` into your project and start tagging consistently.