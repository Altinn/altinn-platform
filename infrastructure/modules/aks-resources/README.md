## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_azapi"></a> [azapi](#requirement\_azapi) | >= 2.3.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_azapi"></a> [azapi](#provider\_azapi) | >= 2.3.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [azapi_resource.cert_manager](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.eso](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.flux_syncroot](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.linkerd](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.otel_collector](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.otel_operator](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.traefik](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_aks_node_resource_group"></a> [aks\_node\_resource\_group](#input\_aks\_node\_resource\_group) | AKS node resource group name | `string` | n/a | yes |
| <a name="input_azurerm_kubernetes_cluster_id"></a> [azurerm\_kubernetes\_cluster\_id](#input\_azurerm\_kubernetes\_cluster\_id) | AKS cluster resource id | `string` | n/a | yes |
| <a name="input_environment"></a> [environment](#input\_environment) | Environment | `string` | n/a | yes |
| <a name="input_flux_release_tag"></a> [flux\_release\_tag](#input\_flux\_release\_tag) | OCI image that Flux should watch and reconcile | `string` | `"latest"` | no |
| <a name="input_obs_client_id"></a> [obs\_client\_id](#input\_obs\_client\_id) | Client id for the obs app | `string` | n/a | yes |
| <a name="input_obs_kv_uri"></a> [obs\_kv\_uri](#input\_obs\_kv\_uri) | Key vault uri for observability | `string` | n/a | yes |
| <a name="input_obs_tenant_id"></a> [obs\_tenant\_id](#input\_obs\_tenant\_id) | Tenant id for the obs app | `string` | n/a | yes |
| <a name="input_pip4_ip_address"></a> [pip4\_ip\_address](#input\_pip4\_ip\_address) | AKS ipv4 public ip | `string` | n/a | yes |
| <a name="input_pip6_ip_address"></a> [pip6\_ip\_address](#input\_pip6\_ip\_address) | AKS ipv6 public ip | `string` | n/a | yes |
| <a name="input_subnet_address_prefixes"></a> [subnet\_address\_prefixes](#input\_subnet\_address\_prefixes) | list of subnets | <pre>object({<br/>    aks_syspool  = list(string)<br/>    aks_workpool = list(string)<br/>  })</pre> | n/a | yes |
| <a name="input_syncroot_namespace"></a> [syncroot\_namespace](#input\_syncroot\_namespace) | The namespace to use for the syncroot. This is the containing 'folder' in altinncr repo and the namespace in the cluster. | `string` | n/a | yes |

## Outputs

No outputs.

<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_azapi"></a> [azapi](#requirement\_azapi) | >= 2.3.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_azapi"></a> [azapi](#provider\_azapi) | >= 2.3.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [azapi_resource.cert_manager](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.cert_manager_issuer](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.container_runtime_aks_config](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.dis_identity_operator](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.eso](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.flux_syncroot](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.grafana_operator](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.lakmus](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.linkerd](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.otel_collector](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.otel_operator](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.traefik](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_aks_node_resource_group"></a> [aks\_node\_resource\_group](#input\_aks\_node\_resource\_group) | AKS node resource group name | `string` | n/a | yes |
| <a name="input_azurerm_dis_identity_resource_group_id"></a> [azurerm\_dis\_identity\_resource\_group\_id](#input\_azurerm\_dis\_identity\_resource\_group\_id) | The resource group ID where the User Assigned Managed Identity managed by dis-identity-operator will be created. | `string` | `""` | no |
| <a name="input_azurerm_kubernetes_cluster_id"></a> [azurerm\_kubernetes\_cluster\_id](#input\_azurerm\_kubernetes\_cluster\_id) | AKS cluster resource id | `string` | n/a | yes |
| <a name="input_azurerm_kubernetes_cluster_oidc_issuer_url"></a> [azurerm\_kubernetes\_cluster\_oidc\_issuer\_url](#input\_azurerm\_kubernetes\_cluster\_oidc\_issuer\_url) | The OIDC issuer URL of the AKS cluster. | `string` | `""` | no |
| <a name="input_developer_entra_id_group"></a> [developer\_entra\_id\_group](#input\_developer\_entra\_id\_group) | EntraID group that should have access to grafana and kubernetes cluster | `string` | n/a | yes |
| <a name="input_enable_cert_manager_tls_issuer"></a> [enable\_cert\_manager\_tls\_issuer](#input\_enable\_cert\_manager\_tls\_issuer) | Enable cert-manager issuer for TLS certificates | `bool` | `true` | no |
| <a name="input_enable_dis_identity_operator"></a> [enable\_dis\_identity\_operator](#input\_enable\_dis\_identity\_operator) | Enable the dis-identity-operator to manage User Assigned Managed Identities in the cluster. | `bool` | `false` | no |
| <a name="input_enable_grafana_operator"></a> [enable\_grafana\_operator](#input\_enable\_grafana\_operator) | Toggle deployment of grafana operator in cluster. If deployed grafana\_endpoint must be defined | `bool` | `true` | no |
| <a name="input_environment"></a> [environment](#input\_environment) | Environment | `string` | n/a | yes |
| <a name="input_flux_release_tag"></a> [flux\_release\_tag](#input\_flux\_release\_tag) | OCI image that Flux should watch and reconcile | `string` | `"latest"` | no |
| <a name="input_grafana_dashboard_release_branch"></a> [grafana\_dashboard\_release\_branch](#input\_grafana\_dashboard\_release\_branch) | Grafana dashboard release branch | `string` | `"release"` | no |
| <a name="input_grafana_endpoint"></a> [grafana\_endpoint](#input\_grafana\_endpoint) | URL endpoint for Grafana dashboard access | `string` | `""` | no |
| <a name="input_grafana_redirect_dns"></a> [grafana\_redirect\_dns](#input\_grafana\_redirect\_dns) | External DNS name used for Grafana redirect; must resolve to the AKS cluster where the Grafana operator is deployed. | `string` | `""` | no |
| <a name="input_lakmus_client_id"></a> [lakmus\_client\_id](#input\_lakmus\_client\_id) | Client id for Lakmus | `string` | n/a | yes |
| <a name="input_linkerd_default_inbound_policy"></a> [linkerd\_default\_inbound\_policy](#input\_linkerd\_default\_inbound\_policy) | Default inbound policy for Linkerd | `string` | `"all-unauthenticated"` | no |
| <a name="input_linkerd_disable_ipv6"></a> [linkerd\_disable\_ipv6](#input\_linkerd\_disable\_ipv6) | Disable IPv6 for Linkerd | `string` | `"false"` | no |
| <a name="input_obs_amw_write_endpoint"></a> [obs\_amw\_write\_endpoint](#input\_obs\_amw\_write\_endpoint) | Azure Monitor Workspaces write endpoint to write prometheus metrics to via prometheus exporter | `string` | n/a | yes |
| <a name="input_obs_client_id"></a> [obs\_client\_id](#input\_obs\_client\_id) | Client id for the obs app | `string` | n/a | yes |
| <a name="input_obs_kv_uri"></a> [obs\_kv\_uri](#input\_obs\_kv\_uri) | Key vault uri for observability | `string` | n/a | yes |
| <a name="input_obs_tenant_id"></a> [obs\_tenant\_id](#input\_obs\_tenant\_id) | Tenant id for the obs app | `string` | n/a | yes |
| <a name="input_pip4_ip_address"></a> [pip4\_ip\_address](#input\_pip4\_ip\_address) | AKS ipv4 public ip | `string` | n/a | yes |
| <a name="input_pip6_ip_address"></a> [pip6\_ip\_address](#input\_pip6\_ip\_address) | AKS ipv6 public ip | `string` | n/a | yes |
| <a name="input_subnet_address_prefixes"></a> [subnet\_address\_prefixes](#input\_subnet\_address\_prefixes) | list of subnets | <pre>object({<br/>    aks_syspool  = list(string)<br/>    aks_workpool = list(string)<br/>  })</pre> | n/a | yes |
| <a name="input_subscription_id"></a> [subscription\_id](#input\_subscription\_id) | Subscription id where aks cluster and other resources are deployed | `string` | n/a | yes |
| <a name="input_syncroot_namespace"></a> [syncroot\_namespace](#input\_syncroot\_namespace) | The namespace to use for the syncroot. This is the containing 'folder' in altinncr repo and the namespace in the cluster. | `string` | n/a | yes |
| <a name="input_tls_cert_manager_workload_identity_client_id"></a> [tls\_cert\_manager\_workload\_identity\_client\_id](#input\_tls\_cert\_manager\_workload\_identity\_client\_id) | Client id for cert-manager workload identity | `string` | `""` | no |
| <a name="input_tls_cert_manager_zone_name"></a> [tls\_cert\_manager\_zone\_name](#input\_tls\_cert\_manager\_zone\_name) | Azure DNS zone name for TLS certificates | `string` | `""` | no |
| <a name="input_tls_cert_manager_zone_rg_name"></a> [tls\_cert\_manager\_zone\_rg\_name](#input\_tls\_cert\_manager\_zone\_rg\_name) | Azure DNS zone resource group name for TLS certificates | `string` | `""` | no |
| <a name="input_token_grafana_operator"></a> [token\_grafana\_operator](#input\_token\_grafana\_operator) | Authentication token for Grafana operator to manage Grafana resources | `string` | `""` | no |

## Outputs

No outputs.
<!-- END_TF_DOCS -->