## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_azapi"></a> [azapi](#requirement\_azapi) | >= 2.3.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_azapi"></a> [azapi](#provider\_azapi) | >= 2.3.0 |

## Resources

| Name | Type |
|------|------|
| [azapi_resource.cert_manager](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.linkerd](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |
| [azapi_resource.traefik](https://registry.terraform.io/providers/Azure/azapi/latest/docs/resources/resource) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_aks_node_resource_group"></a> [aks\_node\_resource\_group](#input\_aks\_node\_resource\_group) | AKS node resource group name | `string` | n/a | yes |
| <a name="input_azurerm_kubernetes_cluster_id"></a> [azurerm\_kubernetes\_cluster\_id](#input\_azurerm\_kubernetes\_cluster\_id) | AKS cluster resource id | `string` | n/a | yes |
| <a name="input_flux_release_tag"></a> [flux\_release\_tag](#input\_flux\_release\_tag) | OCI image that Flux should watch and reconcile | `string` | `"latest"` | no |
| <a name="input_pip4_ip_address"></a> [pip4\_ip\_address](#input\_pip4\_ip\_address) | AKS ipv4 public ip | `string` | n/a | yes |
| <a name="input_pip6_ip_address"></a> [pip6\_ip\_address](#input\_pip6\_ip\_address) | AKS ipv6 public ip | `string` | n/a | yes |
| <a name="input_subnet_address_prefixes"></a> [subnet\_address\_prefixes](#input\_subnet\_address\_prefixes) | list of subnets | <pre>object({<br/>    aks_syspool  = list(string)<br/>    aks_workpool = list(string)<br/>  })</pre> | n/a | yes |
