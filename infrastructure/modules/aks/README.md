## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_azurerm"></a> [azurerm](#requirement\_azurerm) | >= 4.0.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_azurerm"></a> [azurerm](#provider\_azurerm) | >= 4.0.0 |
| <a name="provider_random"></a> [random](#provider\_random) | n/a |

## Resources

| Name | Type |
|------|------|
| [azurerm_kubernetes_cluster.aks](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/kubernetes_cluster) | resource |
| [azurerm_kubernetes_cluster_extension.flux](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/kubernetes_cluster_extension) | resource |
| [azurerm_kubernetes_cluster_node_pool.workpool](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/kubernetes_cluster_node_pool) | resource |
| [azurerm_log_analytics_workspace.aks](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/log_analytics_workspace) | resource |
| [azurerm_monitor_data_collection_endpoint.amw](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_data_collection_endpoint) | resource |
| [azurerm_monitor_data_collection_rule.amw](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_data_collection_rule) | resource |
| [azurerm_monitor_data_collection_rule.law](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_data_collection_rule) | resource |
| [azurerm_monitor_data_collection_rule_association.amw](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_data_collection_rule_association) | resource |
| [azurerm_monitor_data_collection_rule_association.law](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_data_collection_rule_association) | resource |
| [azurerm_monitor_diagnostic_setting.aks](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_diagnostic_setting) | resource |
| [azurerm_monitor_workspace.aks](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/monitor_workspace) | resource |
| [azurerm_public_ip.pip4](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/public_ip) | resource |
| [azurerm_public_ip.pip6](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/public_ip) | resource |
| [azurerm_public_ip_prefix.prefix4](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/public_ip_prefix) | resource |
| [azurerm_public_ip_prefix.prefix6](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/public_ip_prefix) | resource |
| [azurerm_resource_group.aks](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/resource_group) | resource |
| [azurerm_resource_group.monitor](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/resource_group) | resource |
| [azurerm_role_assignment.aks_acrpull](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_role_assignment.network_contributor](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_assignment) | resource |
| [azurerm_storage_account.aks_log](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/storage_account) | resource |
| [azurerm_subnet.aks](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/subnet) | resource |
| [azurerm_virtual_network.aks](https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/virtual_network) | resource |
| [random_id.aks_log](https://registry.terraform.io/providers/hashicorp/random/latest/docs/resources/id) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_admin_group_object_ids"></a> [admin\_group\_object\_ids](#input\_admin\_group\_object\_ids) | List of group object IDs to get admin access to the cluster | `list(string)` | n/a | yes |
| <a name="input_aks_acrpull_scopes"></a> [aks\_acrpull\_scopes](#input\_aks\_acrpull\_scopes) | List of AKS ACR pull scopes | `list(string)` | `[]` | no |
| <a name="input_aks_sku_tier"></a> [aks\_sku\_tier](#input\_aks\_sku\_tier) | Kubernetes SKU | `string` | `"Free"` | no |
| <a name="input_azurerm_kubernetes_cluster_aks_dns_service_ip"></a> [azurerm\_kubernetes\_cluster\_aks\_dns\_service\_ip](#input\_azurerm\_kubernetes\_cluster\_aks\_dns\_service\_ip) | Optional explicit aks dns service ip | `string` | `""` | no |
| <a name="input_azurerm_kubernetes_cluster_aks_name"></a> [azurerm\_kubernetes\_cluster\_aks\_name](#input\_azurerm\_kubernetes\_cluster\_aks\_name) | Optional explicit name of the AKS cluster | `string` | `""` | no |
| <a name="input_azurerm_kubernetes_cluster_aks_pod_cidrs"></a> [azurerm\_kubernetes\_cluster\_aks\_pod\_cidrs](#input\_azurerm\_kubernetes\_cluster\_aks\_pod\_cidrs) | Optional explicit aks service cidrs | `list(string)` | `[]` | no |
| <a name="input_azurerm_kubernetes_cluster_aks_service_cidrs"></a> [azurerm\_kubernetes\_cluster\_aks\_service\_cidrs](#input\_azurerm\_kubernetes\_cluster\_aks\_service\_cidrs) | Optional explicit aks service cidrs | `list(string)` | `[]` | no |
| <a name="input_azurerm_log_analytics_workspace_aks_name"></a> [azurerm\_log\_analytics\_workspace\_aks\_name](#input\_azurerm\_log\_analytics\_workspace\_aks\_name) | Optional explicit name of the log analytics workspace | `string` | `""` | no |
| <a name="input_azurerm_monitor_workspace_aks_name"></a> [azurerm\_monitor\_workspace\_aks\_name](#input\_azurerm\_monitor\_workspace\_aks\_name) | Optional explicit name of the monitor workspace | `string` | `""` | no |
| <a name="input_azurerm_public_ip_prefix_prefix4_name"></a> [azurerm\_public\_ip\_prefix\_prefix4\_name](#input\_azurerm\_public\_ip\_prefix\_prefix4\_name) | Optional explicit name of the public ipv4 prefix | `string` | `""` | no |
| <a name="input_azurerm_public_ip_prefix_prefix6_name"></a> [azurerm\_public\_ip\_prefix\_prefix6\_name](#input\_azurerm\_public\_ip\_prefix\_prefix6\_name) | Optional explicit name of the public ipv6 prefix | `string` | `""` | no |
| <a name="input_azurerm_resource_group_aks_name"></a> [azurerm\_resource\_group\_aks\_name](#input\_azurerm\_resource\_group\_aks\_name) | Optional explicit name of the AKS resource group | `string` | `""` | no |
| <a name="input_azurerm_resource_group_monitor_name"></a> [azurerm\_resource\_group\_monitor\_name](#input\_azurerm\_resource\_group\_monitor\_name) | Optional explicit name of the monitor resource group | `string` | `""` | no |
| <a name="input_azurerm_storage_account_aks_name"></a> [azurerm\_storage\_account\_aks\_name](#input\_azurerm\_storage\_account\_aks\_name) | Optional explicit name of the AKS Log storage account | `string` | `""` | no |
| <a name="input_azurerm_virtual_network_aks_name"></a> [azurerm\_virtual\_network\_aks\_name](#input\_azurerm\_virtual\_network\_aks\_name) | Optional explicit name of the AKS virtual network | `string` | `""` | no |
| <a name="input_azurerm_virtual_public_ip_pip4_name"></a> [azurerm\_virtual\_public\_ip\_pip4\_name](#input\_azurerm\_virtual\_public\_ip\_pip4\_name) | Optional explicit name of the public ipv4 | `string` | `""` | no |
| <a name="input_azurerm_virtual_public_ip_pip6_name"></a> [azurerm\_virtual\_public\_ip\_pip6\_name](#input\_azurerm\_virtual\_public\_ip\_pip6\_name) | Optional explicit name of the public ipv6 | `string` | `""` | no |
| <a name="input_environment"></a> [environment](#input\_environment) | Environment for resources | `string` | n/a | yes |
| <a name="input_kubernetes_version"></a> [kubernetes\_version](#input\_kubernetes\_version) | Kubernetes version | `string` | n/a | yes |
| <a name="input_location"></a> [location](#input\_location) | Default region for resources | `string` | `"norwayeast"` | no |
| <a name="input_pool_configs"></a> [pool\_configs](#input\_pool\_configs) | Variables for node pools | <pre>map(object({<br/>    vm_size              = string<br/>    auto_scaling_enabled = bool<br/>    node_count           = number<br/>    min_count            = number<br/>    max_count            = number<br/>  }))</pre> | n/a | yes |
| <a name="input_prefix"></a> [prefix](#input\_prefix) | Prefix for resource names | `string` | `""` | no |
| <a name="input_subnet_address_prefixes"></a> [subnet\_address\_prefixes](#input\_subnet\_address\_prefixes) | List of subnets | <pre>object({<br/>    aks_syspool  = list(string)<br/>    aks_workpool = list(string)<br/>  })</pre> | n/a | yes |
| <a name="input_subscription_id"></a> [subscription\_id](#input\_subscription\_id) | Subscription ID to deploy services | `string` | n/a | yes |
| <a name="input_vnet_address_space"></a> [vnet\_address\_space](#input\_vnet\_address\_space) | VNet address space | `list(string)` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_aks_identity"></a> [aks\_identity](#output\_aks\_identity) | Managed Service Identity that is configured on this Kubernetes Cluster |
| <a name="output_aks_kubelet_identity"></a> [aks\_kubelet\_identity](#output\_aks\_kubelet\_identity) | Managed Identity assigned to the Kubelets |
| <a name="output_aks_name"></a> [aks\_name](#output\_aks\_name) | The name of the managed Kubernetes Cluster |
| <a name="output_aks_node_resource_group"></a> [aks\_node\_resource\_group](#output\_aks\_node\_resource\_group) | The name of the Resource Group in which the managed Kubernetes Cluster exists |
| <a name="output_aks_oidc_issuer_url"></a> [aks\_oidc\_issuer\_url](#output\_aks\_oidc\_issuer\_url) | The OIDC issuer URL that is associated with the cluster |
| <a name="output_azurerm_kubernetes_cluster_id"></a> [azurerm\_kubernetes\_cluster\_id](#output\_azurerm\_kubernetes\_cluster\_id) | Resource id of aks cluster |
| <a name="output_kube_admin_config"></a> [kube\_admin\_config](#output\_kube\_admin\_config) | Base64 encoded cert/key/user/pass used by clients to authenticate to the Kubernetes cluster |
| <a name="output_pip4_ip_address"></a> [pip4\_ip\_address](#output\_pip4\_ip\_address) | The IPv4 address value that was allocated |
| <a name="output_pip6_ip_address"></a> [pip6\_ip\_address](#output\_pip6\_ip\_address) | The IPv6 address value that was allocated |
