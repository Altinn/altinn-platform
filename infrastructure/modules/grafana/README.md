## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_azuread"></a> [azuread](#requirement\_azuread) | >= 3.0.0 |
| <a name="requirement_azurerm"></a> [azurerm](#requirement\_azurerm) | >= 4.0.0 |
| <a name="requirement_grafana"></a> [grafana](#requirement\_grafana) | >= 3.0.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_azuread"></a> [azuread](#provider\_azuread) | >= 3.0.0 |
| <a name="provider_azurerm"></a> [azurerm](#provider\_azurerm) | >= 4.0.0 |
| <a name="provider_grafana"></a> [grafana](#provider\_grafana) | >= 3.0.0 |
| <a name="provider_http"></a> [http](#provider\_http) | n/a |
| <a name="provider_time"></a> [time](#provider\_time) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [azuread_application.grafana](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application) | resource |
| [azuread_service_principal.grafana](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/service_principal) | resource |
| [azuread_service_principal_password.grafana](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/service_principal_password) | resource |
| [azurerm_dashboard_grafana.grafana](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/dashboard_grafana) | resource |
| [azurerm_resource_group.grafana](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/resource_group) | resource |
| [azurerm_role_assignment.grafana_admin](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [grafana_service_account.admin](https://registry.terraform.io/providers/grafana/grafana/latest/docs/resources/service_account) | resource |
| [grafana_service_account_token.grafana_operator](https://registry.terraform.io/providers/grafana/grafana/latest/docs/resources/service_account_token) | resource |
| [time_rotating.grafana_admin_sp_pwd](https://registry.terraform.io/providers/hashicorp/time/latest/docs/resources/rotating) | resource |
| [http_http.api_key](https://registry.terraform.io/providers/hashicorp/http/latest/docs/data-sources/http) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_client_config_current_object_id"></a> [client\_config\_current\_object\_id](#input\_client\_config\_current\_object\_id) | Object id for pipeline runner id | `string` | n/a | yes |
| <a name="input_environment"></a> [environment](#input\_environment) | Environment for resources | `string` | n/a | yes |
| <a name="input_grafana_major_version"></a> [grafana\_major\_version](#input\_grafana\_major\_version) | Managed Grafana major version. | `number` | `11` | no |
| <a name="input_location"></a> [location](#input\_location) | Default region for resources | `string` | `"norwayeast"` | no |
| <a name="input_prefix"></a> [prefix](#input\_prefix) | Prefix for resource names | `string` | n/a | yes |
| <a name="input_tenant_id"></a> [tenant\_id](#input\_tenant\_id) | Tenant id for resources | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_grafana_bearer_token"></a> [grafana\_bearer\_token](#output\_grafana\_bearer\_token) | n/a |
| <a name="output_grafana_endpoint"></a> [grafana\_endpoint](#output\_grafana\_endpoint) | n/a |
| <a name="output_token_grafana_operator"></a> [token\_grafana\_operator](#output\_token\_grafana\_operator) | n/a |
