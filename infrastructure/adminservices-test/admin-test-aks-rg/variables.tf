variable "subscription_id" {
  type = string
}
variable "admin_services_prod_subscription_id" {
  type = string
}
variable "location" {
  type    = string
  default = "norwayeast"
}
variable "name_prefix" {
  type = string
}
variable "vnet_address_space" {
  type = list(string)
}
variable "subnet_address_prefixes" {
  type = map(list(string))
}
variable "kubernetes_version" {
  type = string
}
variable "aks_sku_tier" {
  type = string
}
variable "pool_configs" {
  type = map(object({
    vm_size   = string
    min_count = string
    max_count = string
  }))
}
variable "flux_release_tag" {
  type = string
}
variable "cert_manager_version" {
  type = string
}