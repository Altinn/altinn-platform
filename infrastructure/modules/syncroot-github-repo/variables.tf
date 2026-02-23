variable "github_repo_name" {
  type        = string
  description = "Name of the Github repo where the syncroot images are going to be pushed from"
}

variable "github_org_name" {
  type        = string
  description = "Name of the Github org where the syncroot images are going to be pushed from"
  default     = "Altinn"
}

variable "github_environments" {
  type        = set(string)
  description = "Github action environments with matching federation"
}

variable "github_refs" {
  type        = set(string)
  description = "Github refs with matching federation"
}

variable "product_name" {
  type        = string
  description = "Name of the team that owns this syncroot user managed identity"
}

variable "subscription_id" {
  type        = string
  description = "subscription where the user manage identity are going to be deployed"
}

variable "location" {
  type        = string
  description = "Azure region where the user assigned managed identity is going to be deployed"
  default     = "norwayeast"
}

variable "tags" {
  type        = map(string)
  description = "Tags to apply to all resources"
  default     = {}
}

variable "resource_group_name" {
  type        = string
  description = "Name of the resourcegroup where the user managed identity is going to be deployed"
}
