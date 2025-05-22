variable "subscription_id" {
  type        = string
  description = "Subscription id to deploy services"
}

variable "subnet_address_prefixes" {
  type = object({
    aks_syspool  = list(string)
    aks_workpool = list(string)
  })
  description = "list of subnets"
}

variable "app_access_token" {
  type        = string
  sensitive   = true
  description = "Azure App access token"
  validation {
    condition     = length(var.app_access_token) > 0
    error_message = "You must provide a value for app_access_token from pipeline run."
  }
}
