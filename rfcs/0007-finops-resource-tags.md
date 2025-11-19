altinn-platform/rfcs/0007-finops-resource-tags.md#L1-160
- Feature Name: `finops_resource_tags`
- Start Date: 2025-11-19
- RFC PR: [altinn/altinn-platform#0007](https://github.com/altinn/altinn-platform/pull/0007)
- Github Issue: [altinn/altinn-platform#0007](https://github.com/altinn/altinn-platform/issues/0007)
- Product/Category: finops
- State: **REVIEW**

# Summary
We define a minimal, consistent set of resource tags for managed Azure resources to make cost allocation and ownership trivial.

# Motivation
Current tagging is inconsistent. We need:
- Reliable cost attribution (environment + product + owner).
- Fast ownership lookup.
- Simple, repeatable tagging pattern.

# Guide-level explanation
All resources get the same base tags. Each resource can add tags describing the component.

## Standard Tag Set (do not rename)
Required in base:
- finops_environment: environment id (dev, test, prod, at22, at23, at24, yt01, tt02)
- finops_product: product name (studio, dialogporten, formidling, autorisasjon, varsling, melding, altinn2)
- finops_serviceownercode: short owner code
- finops_serviceownerorgnr: legal org number (9 digits)
- repository: source repository URL (ex: https://github.com/Altinn/altinn-platform)
- env: duplicate of finops_environment (legacy)
- product: duplicate of finops_product (legacy)
- org: duplicate of finops_serviceownercode (legacy)

## Ownership
- `finops_*` tags are owned by the FinOps team (definition & lifecycle).
- Non-prefixed tags (`env`, `product`, `org`) are owned by the DevOps team (implementation & enforcement).

## Usage Pattern
Define once, reuse everywhere:

Minimal Terraform example:
```hcl
# localtags.tf
locals {
  tags = {
    finops_environment       = var.environment
    finops_product           = "studio"
    finops_serviceownercode  = var.organization
    finops_serviceownerorgnr = var.finops_serviceownerorgnr
    repository               = "https://dev.azure.com/brreg/altinn-apps-ops/_git/${var.organization}"
    env                      = var.environment
    product                  = "studio"
    org                      = var.organization
  }
}
```

Apply to a resource:

```hcl
resource "azurerm_resource_group" "example" {
  name     = "studio-${var.environment}-rg"
  location = var.location
  tags = merge(local.tags, {
    submodule = "my-tf-module"
  })
}
```

Minimal Bicep example:
```bicep
// main.bicep
param environment string
param product string = 'studio'
param serviceOwnerCode string
param serviceOwnerOrgNr string
param repository string

resource rg 'Microsoft.Resources/resourceGroups@2022-09-01' = {
  name: 'studio-${environment}-rg'
  location: 'norwayeast'
  tags: {
    finops_environment: environment
    finops_product: product
    finops_serviceownercode: serviceOwnerCode
    finops_serviceownerorgnr: serviceOwnerOrgNr
    repository: repository
    env: environment
    product: product
    org: serviceOwnerCode
    submodule: 'my-bicep-module'
  }
}
```

## Best Practices
- Always use `merge(local.tags, {...})`.
- Do not duplicate or rewrite the base map.
- Keep tag keys lowercase and exactly as specified.
- Validate tag input variables (e.g. `environment`, `finops_serviceownerorgnr`, product) with Terraform `validation` blocks to catch invalid values early.

Minimal validation example (Terraform):

```hcl
variable "environment" {
  type        = string
  description = "finops_environment tag value"

  validation {
    condition = contains([
      "dev",
      "test",
      "prod",
      "at22",
      "at23",
      "at24",
      "yt01",
      "tt02"
    ], var.environment)
    error_message = "environment must be one of: dev, test, prod, at22, at23, at24, yt01, tt02."
  }
}
```

## Important principles
- Naming convention: All tag names must be English, lowercase, singular
- Values: Short, self-explanatory lowercase values
- Consistency: Same spelling across all resources

# Reference-level explanation
Implementation is a single `locals` map + `merge()` usage. Enforcement can be added later (policy, validation) but is not part of this minimal RFC.

# Drawbacks
- Duplication of environment/product/org.
- Requires discipline to not ad-hoc invent new tags.

# Rationale and alternatives
Alternatives (per-resource custom maps, naming-only encoding) were rejected for inconsistency and poor query ergonomics. A single shared map is simplest.

# Prior art
Aligns with common FinOps guidance: standardized owner + environment + product tags.

# Unresolved questions
- Whether `repository` ever needs to differ per submodule (currently no).
- Definition and scope for a potential `finops_capacity` tag (e.g. sizing class, reserved units, or throughput category) – under discussion, not adopted.

# Future possibilities
Future tags (e.g. cost center, data classification) will be proposed in separate RFCs if/when needed.
