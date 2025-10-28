# Azure Observability Module

A production-ready Terraform module that bootstraps comprehensive Azure observability infrastructure including **Log Analytics Workspace, Application Insights (workspace‑based), Azure Monitor Workspace, Key Vault, and federated identity resources**.

## 🚀 Recent Improvements

This module has been significantly refactored to improve reliability and maintainability:

- **✅ Eliminated Resource Replacement Issues**: Fixed "known after apply" problems using the `try()` pattern
- **✅ Simplified Architecture**: Removed complex local variables and ternary logic  
- **✅ Enhanced Documentation**: Complete variable descriptions and usage examples
- **✅ Production Ready**: Robust validation, proper security practices, and clean code structure
- **✅ Consistent Patterns**: Applied Terraform best practices throughout

## 🏗️ Architecture

The module creates or reuses the following resources:

- **Resource Group**: Container for all observability resources
- **Log Analytics Workspace**: Centralized log collection and analysis
- **Application Insights**: Application performance monitoring (APM)
- **Azure Monitor Workspace**: Prometheus-compatible metrics collection
- **Key Vault**: Secure storage for connection strings and secrets
- **Azure AD Applications**: Federated identity for Kubernetes workloads
- **Data Collection Rules**: Optional AKS cluster monitoring integration

---

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_azuread"></a> [azuread](#requirement\_azuread) | ~> 3.5.0 |
| <a name="requirement_azurerm"></a> [azurerm](#requirement\_azurerm) | >= 4.0.0 |
| <a name="requirement_random"></a> [random](#requirement\_random) | >= 3.7.2 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_azuread"></a> [azuread](#provider\_azuread) | ~> 3.5.0 |
| <a name="provider_azurerm"></a> [azurerm](#provider\_azurerm) | >= 4.0.0 |
| <a name="provider_random"></a> [random](#provider\_random) | >= 3.7.2 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [azuread_application.app](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application) | resource |
| [azuread_application.lakmus_app](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application) | resource |
| [azuread_application_federated_identity_credential.lakmus_fed_identity](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application_federated_identity_credential) | resource |
| [azuread_application_federated_identity_credential.obs_fed_identity](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application_federated_identity_credential) | resource |
| [azuread_service_principal.lakmus_sp](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/service_principal) | resource |
| [azuread_service_principal.sp](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/service_principal) | resource |
| [azurerm_application_insights.obs](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/application_insights) | resource |
| [azurerm_key_vault.obs_kv](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/key_vault) | resource |
| [azurerm_key_vault_secret.conn_string](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/key_vault_secret) | resource |
| [azurerm_log_analytics_workspace.obs](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/log_analytics_workspace) | resource |
| [azurerm_monitor_data_collection_endpoint.amw](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_data_collection_endpoint) | resource |
| [azurerm_monitor_data_collection_rule.amw](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_data_collection_rule) | resource |
| [azurerm_monitor_data_collection_rule_association.amw](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_data_collection_rule_association) | resource |
| [azurerm_monitor_workspace.obs](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_workspace) | resource |
| [azurerm_resource_group.obs](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/resource_group) | resource |
| [azurerm_role_assignment.ci_kv_secrets_role](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.kv_reader_lakmus](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.obs_kv_reader](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [random_string.obs_kv_postfix](https://registry.terraform.io/providers/hashicorp/random/latest/docs/resources/string) | resource |
| [azurerm_application_insights.existing](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/data-sources/application_insights) | data source |
| [azurerm_client_config.current](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/data-sources/client_config) | data source |
| [azurerm_log_analytics_workspace.existing](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/data-sources/log_analytics_workspace) | data source |
| [azurerm_monitor_workspace.existing](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/data-sources/monitor_workspace) | data source |
| [azurerm_resource_group.existing](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/data-sources/resource_group) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_app_insights_app_type"></a> [app\_insights\_app\_type](#input\_app\_insights\_app\_type) | Application type for Application Insights. Common values: web, other. | `string` | `"web"` | no |
| <a name="input_app_insights_name"></a> [app\_insights\_name](#input\_app\_insights\_name) | Name for the Application Insights instance. | `string` | `null` | no |
| <a name="input_azurerm_kubernetes_cluster_id"></a> [azurerm\_kubernetes\_cluster\_id](#input\_azurerm\_kubernetes\_cluster\_id) | AKS cluster resource id | `string` | `""` | no |
| <a name="input_azurerm_resource_group_obs_name"></a> [azurerm\_resource\_group\_obs\_name](#input\_azurerm\_resource\_group\_obs\_name) | Optional explicit name of the observability resource group | `string` | `null` | no |
| <a name="input_enable_aks_monitoring"></a> [enable\_aks\_monitoring](#input\_enable\_aks\_monitoring) | Should monitoring of a AKS cluster be enabled. If true azurerm\_kubernetes\_cluster\_id is required. | `bool` | n/a | yes |
| <a name="input_environment"></a> [environment](#input\_environment) | Environment for resources | `string` | n/a | yes |
| <a name="input_location"></a> [location](#input\_location) | Default region for resources | `string` | `"norwayeast"` | no |
| <a name="input_log_analytics_retention_days"></a> [log\_analytics\_retention\_days](#input\_log\_analytics\_retention\_days) | Number of days to retain logs in Log Analytics Workspace. | `number` | `30` | no |
| <a name="input_log_analytics_workspace_name"></a> [log\_analytics\_workspace\_name](#input\_log\_analytics\_workspace\_name) | Name for the Log Analytics workspace. | `string` | `null` | no |
| <a name="input_monitor_workspace_name"></a> [monitor\_workspace\_name](#input\_monitor\_workspace\_name) | Name for the Azure Monitor workspace. | `string` | `null` | no |
| <a name="input_oidc_issuer_url"></a> [oidc\_issuer\_url](#input\_oidc\_issuer\_url) | Oidc issuer url needed for federation | `string` | n/a | yes |
| <a name="input_prefix"></a> [prefix](#input\_prefix) | Prefix for resource names | `string` | n/a | yes |
| <a name="input_tags"></a> [tags](#input\_tags) | Tags to apply to all resources. | `map(string)` | `{}` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_application_insights_id"></a> [application\_insights\_id](#output\_application\_insights\_id) | The ID of the Application Insights resource. |
| <a name="output_key_vault_uri"></a> [key\_vault\_uri](#output\_key\_vault\_uri) | The URI of the Key Vault for storing secrets. |
| <a name="output_lakmus_client_id"></a> [lakmus\_client\_id](#output\_lakmus\_client\_id) | The client ID of the Lakmus Azure AD application. |
| <a name="output_log_analytics_workspace_id"></a> [log\_analytics\_workspace\_id](#output\_log\_analytics\_workspace\_id) | The ID of the Log Analytics Workspace. |
| <a name="output_monitor_workspace_id"></a> [monitor\_workspace\_id](#output\_monitor\_workspace\_id) | The ID of the Azure Monitor Workspace. |
| <a name="output_obs_client_id"></a> [obs\_client\_id](#output\_obs\_client\_id) | The client ID of the observability Azure AD application. |
<!-- END_TF_DOCS -->
---

## 📋 Usage Examples

### Basic Usage - Create New Resources

```hcl
module "observability" {
  source = "./path/to/observability"

  prefix      = "myapp"
  environment = "production"
  location    = "westeurope"

  # Required for federated identity
  oidc_issuer_url = "https://westeurope.oic.prod-aks.azure.com/00000000/11111111"

  # Optional AKS monitoring
  enable_aks_monitoring         = true
  azurerm_kubernetes_cluster_id = module.aks.aks_id

  tags = {
    project    = "my-application"
    costcenter = "engineering"
    environment = "production"
  }
}
```

### Advanced Usage - Reuse Existing Resources

```hcl
module "observability" {
  source = "./path/to/observability"

  prefix      = "myapp"
  environment = "production"
  location    = "westeurope"

  # Reuse existing resource group
  azurerm_resource_group_obs_name = "shared-observability-rg"

  # Reuse existing Log Analytics workspace
  log_analytics_workspace_name = "shared-law"

  # Create new Application Insights (not specified)
  # Reuse existing Monitor workspace
  monitor_workspace_name = "shared-amw"

  # Required for federated identity
  oidc_issuer_url = "https://westeurope.oic.prod-aks.azure.com/00000000/11111111"

  # Optional AKS monitoring
  enable_aks_monitoring         = true
  azurerm_kubernetes_cluster_id = module.aks.aks_id

  tags = {
    project    = "my-application"
    costcenter = "engineering"
  }
}
```

## 🔧 Configuration Options

### Resource Naming

When creating new resources, the module uses the following naming pattern:
- Resource Group: `{prefix}-{environment}-obs-rg`
- Log Analytics Workspace: `{prefix}-{environment}-obs-law`
- Application Insights: `{prefix}-{environment}-obs-appinsights`
- Monitor Workspace: `{prefix}-{environment}-obs-amw`
- Key Vault: `obs-{prefix}-{environment}-{random-postfix}` (24 chars max)

### Resource Reuse

To reuse existing resources, simply provide the resource names:
- Set `azurerm_resource_group_obs_name` to reuse an existing resource group
- Set `log_analytics_workspace_name` to reuse an existing Log Analytics workspace
- Set `app_insights_name` to reuse an existing Application Insights
- Set `monitor_workspace_name` to reuse an existing Monitor workspace

### AKS Integration

When `enable_aks_monitoring = true`, the module creates:
- Data Collection Endpoint for metrics collection
- Data Collection Rules for Prometheus metrics
- Association between the AKS cluster and monitoring infrastructure

## 🔐 Security Considerations

- **Key Vault**: RBAC authorization enabled with purge protection
- **Federated Identity**: Uses OIDC federation for Kubernetes workloads
- **Role Assignments**: Minimal required permissions granted
- **Secret Management**: Connection strings stored securely in Key Vault

## 🚀 Quick Start

```hcl
module "observability" {
  source      = "../some/path"

  prefix      = "acme"
  environment = "dev"
  location    = "westeurope"

  # Reuse existing resources in the given RG
  azurerm_resource_group_obs_name = "shared-observability-rg"

  # if not passed a new resource will be created in the RG
  log_analytics_workspace_name = "shared-law" #reused
  # an app_insights will be created as it not being passed
  monitor_workspace_name       = "shared-amw"

  # Enable monitoring for an existing AKS cluster
  enable_aks_monitoring         = true
  azurerm_kubernetes_cluster_id = module.aks.aks_id
  oidc_issuer_url = "https://westeurope.oic.prod-aks.azure.com/00000000/11111111"

  # Object ID of the service principal that needs read/write access to secrets
  # NOTE: This is the AAD objectId, not the appId/clientId.
  # This is no longer required as the module handles role assignments automatically
  # pipeline_sp_object_id = "00000000-0000-0000-0000-000000000000"

  tags = {
    project    = "billing-api"
    costcenter = "42"
  }
}
```

## 🔧 Technical Improvements

### Resource Management Pattern

The module now uses the `try()` pattern for seamless resource reuse:

```hcl
# Before: Complex ternary logic
resource_group_name = local.reuse_rg ? var.resource_group_name : local.rg.name

# After: Clean try() pattern  
resource_group_name = try(azurerm_resource_group.obs[0].name, var.azurerm_resource_group_obs_name)
```

**Benefits:**
- ✅ Eliminates "known after apply" issues
- ✅ Prevents unnecessary resource replacements
- ✅ Simplifies code and improves readability
- ✅ Follows Terraform best practices

### Migration from Previous Versions

If you're upgrading from a previous version of this module:

1. **No Breaking Changes**: All existing configurations will continue to work
2. **Improved Reliability**: Resource replacement issues have been resolved
3. **Cleaner Plans**: Terraform plans should now show fewer changes
4. **Better Performance**: Reduced complexity improves execution time

### Validation Enhancements

The module includes comprehensive input validation:

```hcl
# Required variables with validation
variable "environment" {
  type        = string
  description = "Environment for resources"
  validation {
    condition     = length(var.environment) > 0
    error_message = "You must provide a value for environment."
  }
}

# Conditional validation
variable "azurerm_kubernetes_cluster_id" {
  validation {
    condition     = var.enable_aks_monitoring == false || (var.enable_aks_monitoring == true && length(var.azurerm_kubernetes_cluster_id) > 0)
    error_message = "You must provide a value for azurerm_kubernetes_cluster_id when enable_aks_monitoring is true."
  }
}
```

## 🐛 Troubleshooting

### Common Issues and Solutions

**Issue: Resource replacement on apply**
```
# module.observability.azurerm_key_vault.obs_kv must be replaced
```
**Solution**: This has been fixed in the current version. The `try()` pattern eliminates this issue.

**Issue: "known after apply" errors**
**Solution**: Ensure you're using the latest version of the module with the refactored resource management.

**Issue: AKS monitoring not working**
**Solution**: Verify that:
- `enable_aks_monitoring = true`
- `azurerm_kubernetes_cluster_id` is correctly set
- `oidc_issuer_url` matches your AKS cluster's OIDC endpoint

## 📚 Additional Resources

- [Azure Monitor Documentation](https://docs.microsoft.com/en-us/azure/azure-monitor/)
- [Application Insights Overview](https://docs.microsoft.com/en-us/azure/azure-monitor/app/app-insights-overview)
- [Azure Key Vault Best Practices](https://docs.microsoft.com/en-us/azure/key-vault/general/best-practices)
- [Terraform try() Function](https://www.terraform.io/language/functions/try)

## 🤝 Contributing

This module follows Terraform best practices. When contributing:
- Use the `try()` pattern for conditional resource references
- Maintain consistent naming conventions
- Add comprehensive variable descriptions
- Include proper validation for all inputs
- Follow the established file structure

---

**Version**: 2.0.0 (Major Refactor)  
**Last Updated**: 2024  
**Compatibility**: Terraform >= 1.0, AzureRM Provider >= 4.0.0
