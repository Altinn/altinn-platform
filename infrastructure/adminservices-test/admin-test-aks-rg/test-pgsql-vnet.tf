module "test-pgsql-vnet" {
  source = "../../modules/postgresql-vnet-subnets"
  name = "test-pgsql-vnet"
  resource_group_name = module.aks.dis_resource_group_name
  location = "norwayeast"
  vnet_address_space = "${var.pgsql_vnet_address_space}"
  peered_vnets = [{
    id = module.aks.aks_workpool_vnet_id
    name = module.aks.aks_workpool_vnet_name
    resource_group_name = module.aks.aks_workpool_vnet_resource_group_name
  }]
  tags = {}
}
