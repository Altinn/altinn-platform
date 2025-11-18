<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_azuread"></a> [azuread](#requirement\_azuread) | >= 3.1.0 |
| <a name="requirement_azurerm"></a> [azurerm](#requirement\_azurerm) | >= 4.0.0 |
| <a name="requirement_grafana"></a> [grafana](#requirement\_grafana) | >= 3.0.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_azurerm"></a> [azurerm](#provider\_azurerm) | >= 4.0.0 |
| <a name="provider_grafana"></a> [grafana](#provider\_grafana) | >= 3.0.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [azurerm_dashboard_grafana.grafana](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/dashboard_grafana) | resource |
| [azurerm_resource_group.grafana](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/resource_group) | resource |
| [azurerm_role_assignment.amw_datareaderrole](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.grafana_admin](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.grafana_admin_sp](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.grafana_editor](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.grafana_permission](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [grafana_service_account.admin](https://registry.terraform.io/providers/grafana/grafana/latest/docs/resources/service_account) | resource |
| [grafana_service_account_token.grafana_operator](https://registry.terraform.io/providers/grafana/grafana/latest/docs/resources/service_account_token) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_client_config_current_object_id"></a> [client\_config\_current\_object\_id](#input\_client\_config\_current\_object\_id) | Object id for pipeline runner id | `string` | n/a | yes |
| <a name="input_create_resource_group"></a> [create\_resource\_group](#input\_create\_resource\_group) | Whether to create a new resource group. If false, will use an existing resource group specified by resource\_group\_name. | `bool` | `true` | no |
| <a name="input_dashboard_name"></a> [dashboard\_name](#input\_dashboard\_name) | Name of Grafana dashboard. If not provided, generates 'grafana-{prefix}-{environment}'. | `string` | `""` | no |
| <a name="input_environment"></a> [environment](#input\_environment) | Environment for resources | `string` | n/a | yes |
| <a name="input_grafana_admin_access"></a> [grafana\_admin\_access](#input\_grafana\_admin\_access) | List of user groups to grant admin access to grafana. | `list(string)` | `[]` | no |
| <a name="input_grafana_editor_access"></a> [grafana\_editor\_access](#input\_grafana\_editor\_access) | List of user groups to grant editor access to grafana. | `list(string)` | `[]` | no |
| <a name="input_grafana_major_version"></a> [grafana\_major\_version](#input\_grafana\_major\_version) | Managed Grafana major version. | `number` | `11` | no |
| <a name="input_grafana_monitor_reader_subscription_id"></a> [grafana\_monitor\_reader\_subscription\_id](#input\_grafana\_monitor\_reader\_subscription\_id) | List of subscription ids to grant reader access to grafana. | `list(string)` | `[]` | no |
| <a name="input_localtags"></a> [localtags](#input\_localtags) | A map of tags to assign to the created resources. | `map(string)` | `{}` | no |
| <a name="input_location"></a> [location](#input\_location) | Default region for resources | `string` | `"norwayeast"` | no |
| <a name="input_monitor_workspace_ids"></a> [monitor\_workspace\_ids](#input\_monitor\_workspace\_ids) | List of azure monitor workspaces to connect grafana. | `map(string)` | `{}` | no |
| <a name="input_prefix"></a> [prefix](#input\_prefix) | Prefix for resource names | `string` | n/a | yes |
| <a name="input_resource_group_name"></a> [resource\_group\_name](#input\_resource\_group\_name) | Name of the resource group. When create\_resource\_group is true, uses this name if provided, otherwise generates 'grafana-{prefix}-{environment}-rg'. When create\_resource\_group is false, this is required and must be the name of an existing resource group. | `string` | `""` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_grafana_endpoint"></a> [grafana\_endpoint](#output\_grafana\_endpoint) | n/a |
| <a name="output_token_grafana_operator"></a> [token\_grafana\_operator](#output\_token\_grafana\_operator) | n/a |
<!-- END_TF_DOCS -->