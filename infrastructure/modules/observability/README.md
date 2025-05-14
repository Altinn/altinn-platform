# dis-observability-module

A lightweight Terraform module that bootstraps a **Log Analytics Workspace, Application Insights (workspace‑based), Azure Monitor Workspace**.

---

## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_azurerm"></a> [azurerm](#requirement_azurerm) | >= 4.0.0 |
| <a name="requirement_azuread"></a> [azuread](#requirement_azuread) | >= 3.1.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_azurerm"></a> [azurerm](#provider_azurerm) | >= 4.0.0 |
| <a name="provider_azuread"></a> [kubernetes](#provider_azuread) | >= 3.1.0 |

## Resources

| Name | Type |
|------|------|
| [azurerm_log_analytics_workspace.obs](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/log_analytics_workspace) | resource |
| [azurerm_application_insights.obs](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/application_insights) | resource |
| [azurerm_monitor_workspace.obs](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_workspace) | resource |
| [azurerm_resource_group.obs](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/resource_group) | resource |
| [kubernetes_secret.app_insights_conn](https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs/resources/secret) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_prefix"></a> [prefix](#input_prefix) | Prefix for generated resource names. | `string` | `""` | **yes** |
| <a name="input_environment"></a> [environment](#input_environment) | Environment identifier (`dev`, `prod`, …). | `string` | `""` | **yes** |
| <a name="input_oidc_issuer_url"></a> [oidc_issuer_url](#input_oidc_issuer_url) | oidc issuer url from AKS. | `string` | `""` | **yes** |
| <a name="input_location"></a> [location](#input_location) | Azure region for all resources. | `string` | `"norwayeast"` | no |
| <a name="input_azurerm_resource_group_obs_name"></a> [azurerm_resource_group_obs_name](#input_azurerm_resource_group_obs_name) | Explicit name of the observability resource‑group (leave empty to let the module create one). | `string` | `""` | no |
| <a name="input_log_analytics_workspace_name"></a> [log_analytics_workspace_name](#input_log_analytics_workspace_name) | Custom name for the Log Analytics Workspace. | `string` | `""` | no |
| <a name="input_log_analytics_retention_days"></a> [log_analytics_retention_days](#input_log_analytics_retention_days) | Retention (days) for Log Analytics. | `number` | `30` | no |
| <a name="input_app_insights_name"></a> [app_insights_name](#input_app_insights_name) | Custom name for Application Insights. | `string` | `""` | no |
| <a name="input_app_insights_app_type"></a> [app_insights_app_type](#input_app_insights_app_type) | Application Insights `application_type`. | `string` | `"web"` | no |
| <a name="input_monitor_workspace_name"></a> [monitor_workspace_name](#input_monitor_workspace_name) | Custom name for Azure Monitor Workspace. | `string` | `""` | no |
| <a name="input_kubeconfig_path"></a> [kubeconfig_path](#input_kubeconfig_path) | Path to kubeconfig that reaches the target cluster. | `string` | `"~/.kube/config"` | no |
| <a name="input_kube_context"></a> [kube_context](#input_kube_context) | Kube‑context to select (defaults to current). | `string` | `""` | no |
| <a name="input_tags"></a> [tags](#input_tags) | Map of tags applied to every Azure resource. | `map(string)` | `{}` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_log_analytics_workspace_id"></a> [log_analytics_workspace_id](#output_log_analytics_workspace_id) | Resource ID of the Log Analytics Workspace. |
| <a name="output_app_insights_id"></a> [app_insights_id](#output_app_insights_id) | Resource ID of the Application Insights. |
| <a name="output_monitor_workspace_id"></a> [monitor_workspace_id](#output_monitor_workspace_id) | Resource ID of the Azure Monitor Workspace. |

---

## Quick start

```hcl
module "observability" {
  source      = "../some/path"

  prefix      = "acme"
  environment = "dev"
  location    = "westeurope"

  oidc_issuer_url = "https://westeurope.oic.prod-aks.azure.com/00000000-0000-0000-0000-000000000000/11111111-1111-1111-1111-111111111111/"

  tags = {
    project    = "billing-api"
    costcenter = "42"
  }
}
