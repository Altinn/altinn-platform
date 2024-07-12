variable "configuration_file" {
  type        = string
  description = "YAML file that contains all organization configuration"
  default     = "../../products.yaml"
  nullable    = false
}

variable "workspaces" {
  type = list(object({
    name = string
    environments = list(object({
      arm_subscription = string
      names            = list(string)
    }))
  }))

  nullable = false
}

variable "arm_resource_group_name" {
  type     = string
  default  = "terraform-rg"
  nullable = false
}

variable "arm_solution_name" {
  type     = string
  default  = "terraform"
  nullable = false
}

variable "arm_product_name" {
  type     = string
  default  = "altinn"
  nullable = false
}

variable "arm_instance" {
  type    = string
  default = "02"
}

variable "arm_billing_account_name" {
  default  = null
  nullable = true
  type     = string
}

variable "arm_enrollment_account_scope" {
  default  = null
  nullable = true
  type     = string
}
