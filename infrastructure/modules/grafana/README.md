<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
| ---- | ------- |
| <a name="requirement_azapi"></a> [azapi](#requirement\_azapi) | >= 2.0.0 |
| <a name="requirement_azuread"></a> [azuread](#requirement\_azuread) | >= 3.1.0 |
| <a name="requirement_azurerm"></a> [azurerm](#requirement\_azurerm) | >= 4.0.0 |
| <a name="requirement_grafana"></a> [grafana](#requirement\_grafana) | >= 3.0.0 |

## Providers

| Name | Version |
| ---- | ------- |
| <a name="provider_azapi"></a> [azapi](#provider\_azapi) | >= 2.0.0 |
| <a name="provider_azuread"></a> [azuread](#provider\_azuread) | >= 3.1.0 |
| <a name="provider_azurerm"></a> [azurerm](#provider\_azurerm) | >= 4.0.0 |
| <a name="provider_grafana"></a> [grafana](#provider\_grafana) | >= 3.0.0 |
| <a name="provider_time"></a> [time](#provider\_time) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
| ---- | ---- |
| [azapi_resource.smtp_username](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azuread_application.smtp](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application) | resource |
| [azuread_application_password.smtp](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/application_password) | resource |
| [azuread_service_principal.smtp](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/resources/service_principal) | resource |
| [azurerm_dashboard_grafana.grafana](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/dashboard_grafana) | resource |
| [azurerm_resource_group.grafana](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/resource_group) | resource |
| [azurerm_role_assignment.amw_datareaderrole](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.grafana_admin](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.grafana_admin_sp](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.grafana_editor](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.grafana_permission](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.smtp_sender](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [grafana_service_account.admin](https://registry.terraform.io/providers/grafana/grafana/latest/docs/resources/service_account) | resource |
| [grafana_service_account_token.grafana_operator](https://registry.terraform.io/providers/grafana/grafana/latest/docs/resources/service_account_token) | resource |
| [time_rotating.grafana_operator_token](https://registry.terraform.io/providers/hashicorp/time/latest/docs/resources/rotating) | resource |
| [time_rotating.smtp_secret](https://registry.terraform.io/providers/hashicorp/time/latest/docs/resources/rotating) | resource |
| [azuread_client_config.current](https://registry.terraform.io/providers/hashicorp/azuread/latest/docs/data-sources/client_config) | data source |

## Inputs

| Name | Description | Type | Default | Required |
| ---- | ----------- | ---- | ------- | :------: |
| <a name="input_client_config_current_object_id"></a> [client\_config\_current\_object\_id](#input\_client\_config\_current\_object\_id) | Object id for pipeline runner id | `string` | n/a | yes |
| <a name="input_create_resource_group"></a> [create\_resource\_group](#input\_create\_resource\_group) | Whether to create a new resource group. If false, will use an existing resource group specified by resource\_group\_name. | `bool` | `true` | no |
| <a name="input_dashboard_name"></a> [dashboard\_name](#input\_dashboard\_name) | Name of Grafana dashboard. If not provided, generates 'grafana-{prefix}-{environment}'. | `string` | `""` | no |
| <a name="input_environment"></a> [environment](#input\_environment) | Environment for resources | `string` | n/a | yes |
| <a name="input_grafana_admin_access"></a> [grafana\_admin\_access](#input\_grafana\_admin\_access) | List of user groups to grant admin access to grafana. | `list(string)` | `[]` | no |
| <a name="input_grafana_editor_access"></a> [grafana\_editor\_access](#input\_grafana\_editor\_access) | List of user groups to grant editor access to grafana. | `list(string)` | `[]` | no |
| <a name="input_grafana_major_version"></a> [grafana\_major\_version](#input\_grafana\_major\_version) | Managed Grafana major version. | `number` | `12` | no |
| <a name="input_grafana_monitor_reader_subscription_id"></a> [grafana\_monitor\_reader\_subscription\_id](#input\_grafana\_monitor\_reader\_subscription\_id) | List of subscription ids to grant reader access to grafana. | `list(string)` | `[]` | no |
| <a name="input_grafana_operator_token_expiration_days"></a> [grafana\_operator\_token\_expiration\_days](#input\_grafana\_operator\_token\_expiration\_days) | Lifetime in days for the grafana-operator service account token. Must be less than or equal to the Grafana instance's service\_accounts.token\_expiration\_day\_limit. | `number` | `360` | no |
| <a name="input_grafana_operator_token_rotation_days"></a> [grafana\_operator\_token\_rotation\_days](#input\_grafana\_operator\_token\_rotation\_days) | Number of days after which the grafana-operator service account token is rotated. Must be less than grafana\_operator\_token\_expiration\_days so the token is recreated before it expires. | `number` | `180` | no |
| <a name="input_localtags"></a> [localtags](#input\_localtags) | A map of tags to assign to the created resources. | `map(string)` | `{}` | no |
| <a name="input_location"></a> [location](#input\_location) | Default region for resources | `string` | `"norwayeast"` | no |
| <a name="input_monitor_workspace_ids"></a> [monitor\_workspace\_ids](#input\_monitor\_workspace\_ids) | List of azure monitor workspaces to connect grafana. | `map(string)` | `{}` | no |
| <a name="input_prefix"></a> [prefix](#input\_prefix) | Prefix for resource names | `string` | n/a | yes |
| <a name="input_resource_group_name"></a> [resource\_group\_name](#input\_resource\_group\_name) | Name of the resource group. When create\_resource\_group is true, uses this name if provided, otherwise generates 'grafana-{prefix}-{environment}-rg'. When create\_resource\_group is false, this is required and must be the name of an existing resource group. | `string` | `""` | no |
| <a name="input_smtp"></a> [smtp](#input\_smtp) | Send Grafana email through the dis-acs-relay Azure Communication Services SMTP relay. Disabled when null.<br/>Creates an Entra app with a rotating client secret and registers it as the SMTP username on the relay.<br/>The deploying identity needs the ACS Terraform Operations (dis-acs-relay) role on the relay.<br/>  communication\_service\_id  - communication\_service\_id output of dis-email-relay-acs-rg<br/>  sender\_role\_definition\_id - smtp\_sender\_role\_definition\_id output of dis-email-relay-acs-rg<br/>  from\_address              - must be a sender registered on the relay | <pre>object({<br/>    communication_service_id  = string<br/>    sender_role_definition_id = string<br/>    from_address              = optional(string, "grafana@altinn.cloud")<br/>    from_name                 = optional(string, "Altinn Grafana")<br/>    secret_rotation_days      = optional(number, 180)<br/>  })</pre> | `null` | no |

## Outputs

| Name | Description |
| ---- | ----------- |
| <a name="output_grafana_endpoint"></a> [grafana\_endpoint](#output\_grafana\_endpoint) | n/a |
| <a name="output_smtp_application_client_id"></a> [smtp\_application\_client\_id](#output\_smtp\_application\_client\_id) | n/a |
| <a name="output_token_grafana_operator"></a> [token\_grafana\_operator](#output\_token\_grafana\_operator) | n/a |
<!-- END_TF_DOCS -->