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

variable "grafana_access_token" {
  type        = string
  description = "Azure Grafana access token"
  validation {
    condition     = length(var.grafana_access_token) > 0
    error_message = "You must provide a value for grafana_access_token from pipeline run."
  }
}
