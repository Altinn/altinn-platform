variable "namespaces" {
  type        = list(string)
  default     = []
  description = "Namespaces to include in the azurerm_monitor_data_collection_rule dataCollectionSettings"
}

variable "k8s_admin_group_object_ids" {
  type        = list(string)
  default     = ["c9c317cc-aec0-4c8b-bdad-b54333686e8a"]
  description = "A list of Object IDs of Azure Active Directory Groups which should have Admin Role on the Cluster."
}
variable "k8s_users_group_object_id" {
  type        = string
  default     = "b95b1fc9-7f21-49c3-8932-07161cd9ac5a"
  description = "Group to assign the Azure Kubernetes Service Cluster User Role to"
}

# TODO: check how many logs we are generating and tweak accordingly
variable "log_analytics_workspace_daily_quota_gb" {
  type        = number
  default     = 5
  description = "The workspace daily quota for ingestion in GB."
}

variable "log_analytics_workspace_retention_in_days" {
  type        = number
  default     = 30
  description = "The workspace data retention in days. Possible values are between 30 and 730"
}
