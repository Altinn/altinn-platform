# observability-module

A lightweight Terraform module that bootstraps a **Log Analytics Workspace, Application Insights (workspace‑based), Azure Monitor Workspace**.

---

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.9.0 |
| <a name="requirement_azuread"></a> [azuread](#requirement\_azuread) | >= 3.6.0 |
| <a name="requirement_azurerm"></a> [azurerm](#requirement\_azurerm) | >= 4.42.0 |
| <a name="requirement_random"></a> [random](#requirement\_random) | >= 3.7.2 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_azuread"></a> [azuread](#provider\_azuread) | >= 3.6.0 |
| <a name="provider_azurerm"></a> [azurerm](#provider\_azurerm) | >= 4.42.0 |
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
| [azurerm_monitor_alert_prometheus_rule_group.kubernetes_recording_rules_linux](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_alert_prometheus_rule_group) | resource |
| [azurerm_monitor_alert_prometheus_rule_group.node_recording_rules_linux](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_alert_prometheus_rule_group) | resource |
| [azurerm_monitor_alert_prometheus_rule_group.ux_recording_rules_linux](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_alert_prometheus_rule_group) | resource |
| [azurerm_monitor_data_collection_endpoint.amw](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_data_collection_endpoint) | resource |
| [azurerm_monitor_data_collection_rule.amw](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_data_collection_rule) | resource |
| [azurerm_monitor_data_collection_rule_association.amw](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_data_collection_rule_association) | resource |
| [azurerm_monitor_workspace.obs](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_workspace) | resource |
| [azurerm_resource_group.obs](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/resource_group) | resource |
| [azurerm_role_assignment.ci_kv_secrets_role](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.kv_reader_lakmus](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.obs_kv_reader](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.otel_collector_metrics_publisher](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [random_string.obs_kv_postfix](https://registry.terraform.io/providers/hashicorp/random/latest/docs/resources/string) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_app_insights_app_type"></a> [app\_insights\_app\_type](#input\_app\_insights\_app\_type) | Application type for Application Insights. Common values: web, other. | `string` | `"web"` | no |
| <a name="input_app_insights_connection_string"></a> [app\_insights\_connection\_string](#input\_app\_insights\_connection\_string) | Connection string of an existing Application Insights when reusing. | `string` | `null` | no |
| <a name="input_azurerm_kubernetes_cluster_id"></a> [azurerm\_kubernetes\_cluster\_id](#input\_azurerm\_kubernetes\_cluster\_id) | AKS cluster resource id | `string` | `null` | no |
| <a name="input_azurerm_resource_group_obs_name"></a> [azurerm\_resource\_group\_obs\_name](#input\_azurerm\_resource\_group\_obs\_name) | Name of the existing observability resource group. If provided, the module will use this resource instead of creating a new one. | `string` | `null` | no |
| <a name="input_ci_service_principal_object_id"></a> [ci\_service\_principal\_object\_id](#input\_ci\_service\_principal\_object\_id) | Object ID of the CI service principal used for role assignments. | `string` | n/a | yes |
| <a name="input_enable_aks_monitoring"></a> [enable\_aks\_monitoring](#input\_enable\_aks\_monitoring) | Should monitoring of a AKS cluster be enabled. If true azurerm\_kubernetes\_cluster\_id is required. | `bool` | n/a | yes |
| <a name="input_environment"></a> [environment](#input\_environment) | Environment for resources | `string` | n/a | yes |
| <a name="input_localtags"></a> [localtags](#input\_localtags) | A map of tags to assign to the created resources. | `map(string)` | `{}` | no |
| <a name="input_location"></a> [location](#input\_location) | Default region for resources | `string` | `"norwayeast"` | no |
| <a name="input_log_analytics_retention_days"></a> [log\_analytics\_retention\_days](#input\_log\_analytics\_retention\_days) | Number of days to retain logs in Log Analytics Workspace. | `number` | `30` | no |
| <a name="input_log_analytics_workspace_id"></a> [log\_analytics\_workspace\_id](#input\_log\_analytics\_workspace\_id) | ID of an existing Log Analytics Workspace when reusing. | `string` | `null` | no |
| <a name="input_monitor_workspace_id"></a> [monitor\_workspace\_id](#input\_monitor\_workspace\_id) | ID of an existing Azure Monitor Workspace when reusing. | `string` | `null` | no |
| <a name="input_monitor_workspace_name"></a> [monitor\_workspace\_name](#input\_monitor\_workspace\_name) | Name of an existing Azure Monitor Workspace when reusing. | `string` | `null` | no |
| <a name="input_oidc_issuer_url"></a> [oidc\_issuer\_url](#input\_oidc\_issuer\_url) | Oidc issuer url needed for federation | `string` | n/a | yes |
| <a name="input_prefix"></a> [prefix](#input\_prefix) | Prefix for resource names | `string` | n/a | yes |
| <a name="input_reuse_application_insights"></a> [reuse\_application\_insights](#input\_reuse\_application\_insights) | Set true to reuse an existing Application Insights instance (pass app\_insights\_connection\_string). | `bool` | `false` | no |
| <a name="input_reuse_log_analytics_workspace"></a> [reuse\_log\_analytics\_workspace](#input\_reuse\_log\_analytics\_workspace) | Set true to reuse an existing Log Analytics Workspace (pass log\_analytics\_workspace\_id). | `bool` | `false` | no |
| <a name="input_reuse_monitor_workspace"></a> [reuse\_monitor\_workspace](#input\_reuse\_monitor\_workspace) | Set true to reuse an existing Azure Monitor Workspace (pass monitor\_workspace\_name and monitor\_workspace\_id). | `bool` | `false` | no |
| <a name="input_subscription_id"></a> [subscription\_id](#input\_subscription\_id) | Azure subscription ID for resource deployments. | `string` | n/a | yes |
| <a name="input_tenant_id"></a> [tenant\_id](#input\_tenant\_id) | Azure AD tenant ID for resource configuration. | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_application_insights_id"></a> [application\_insights\_id](#output\_application\_insights\_id) | n/a |
| <a name="output_key_vault_uri"></a> [key\_vault\_uri](#output\_key\_vault\_uri) | n/a |
| <a name="output_kubernetes_recording_rules_id"></a> [kubernetes\_recording\_rules\_id](#output\_kubernetes\_recording\_rules\_id) | ID of the Kubernetes Recording Rules rule group |
| <a name="output_lakmus_client_id"></a> [lakmus\_client\_id](#output\_lakmus\_client\_id) | n/a |
| <a name="output_log_analytics_workspace_id"></a> [log\_analytics\_workspace\_id](#output\_log\_analytics\_workspace\_id) | n/a |
| <a name="output_monitor_workspace_id"></a> [monitor\_workspace\_id](#output\_monitor\_workspace\_id) | n/a |
| <a name="output_monitor_workspace_write_endpoint"></a> [monitor\_workspace\_write\_endpoint](#output\_monitor\_workspace\_write\_endpoint) | Metrics ingestion endpoint url. If enable\_aks\_monitoring is set to false this will return an empty string |
| <a name="output_node_recording_rules_id"></a> [node\_recording\_rules\_id](#output\_node\_recording\_rules\_id) | ID of the Node Recording Rules rule group |
| <a name="output_obs_client_id"></a> [obs\_client\_id](#output\_obs\_client\_id) | n/a |
| <a name="output_ux_recording_rules_id"></a> [ux\_recording\_rules\_id](#output\_ux\_recording\_rules\_id) | ID of the UX Recording Rules rule group |
<!-- END_TF_DOCS -->
---

## Quick start

```hcl
module "observability" {
  source      = "../some/path"

  prefix      = "acme"
  environment = "dev"
  location    = "norwayeast"
  tenant_id        = "00000000-0000-0000-0000-000000000000"
  subscription_id  = "11111111-1111-1111-1111-111111111111"

  # Reuse existing resources in the given RG
  azurerm_resource_group_obs_name = "shared-observability-rg"

  # if not passed a new resource will be created in the RG
  log_analytics_workspace_name = "shared-law" #reused
  # an app_insights will be created as it is not being passed
  monitor_workspace_name       = "shared-amw"

  # Enable monitoring for an existing AKS cluster
  enable_aks_monitoring         = true
  azurerm_kubernetes_cluster_id = module.aks.aks_id
  oidc_issuer_url = "https://norwayeast.oic.prod-aks.azure.com/00000000/11111111"

  # Object ID of the service principal that needs read/write access to secrets
  # NOTE: This is the AAD objectId, not the appId/clientId.
  ci_service_principal_object_id = "00000000-0000-0000-0000-000000000000"

  localtags = {
    owner      = "platform"
    visibility = "shared"
  }
}
```