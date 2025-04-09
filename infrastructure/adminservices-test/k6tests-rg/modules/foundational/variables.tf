variable "suffix" {
  type = string
}

variable "tenant_id" {
  type = string
}

variable "namespaces" {
  type        = list(string)
  default     = []
  description = "Namespaces to include in the azurerm_monitor_data_collection_rule dataCollectionSettings"
}

variable "k8s_admin_group_object_ids" {
  type        = list(string)
  description = "A list of Object IDs of Azure Active Directory Groups which should have Admin Role on the Cluster."
}
variable "k8s_users_group_object_id" {
  type        = string
  description = "Group to assign the Azure Kubernetes Service Cluster User Role to"
}

variable "log_analytics_workspace_daily_quota_gb" {
  type        = number
  description = "The workspace daily quota for ingestion in GB."
}

variable "log_analytics_workspace_retention_in_days" {
  type        = number
  description = "The workspace data retention in days. Possible values are between 30 and 730"
}
