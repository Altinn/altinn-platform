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
  type    = string
  default = "altinn-apim-test"
}