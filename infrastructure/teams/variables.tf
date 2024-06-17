variable "configuration_file" {
  type        = string
  description = "YAML file that contains all organization configuration"
  default     = "../../organization.yaml"
  nullable    = false
}

variable "environments" {
  type = list(object({
    name = string
    workspaces = list(object({
      arm_subscription = string
      names            = list(string)
    }))
  }))

  nullable = false
}

variable "arm_location" {
  type    = string
  default = "norwayeast"
}

variable "arm_instance" {
  type    = string
  default = "002"
  validation {
    error_message = "instance must be between [001, 999]"
    condition     = can(regex("^(00[1-9]|0[1-9][0-9]|[1-9][0-9]{2})$", var.arm_instance))
  }
}

variable "arm_solution_name" {
  type     = string
  default  = "tfstate"
  nullable = false
}

variable "arm_product_name" {
  type     = string
  default  = "altinn"
  nullable = false
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
