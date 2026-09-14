variable "github_repo_name" {
  type        = string
  description = "Name of the Github repo where the syncroot images are going to be pushed from"
}

variable "github_org_name" {
  type        = string
  description = "Name of the Github org where the syncroot images are going to be pushed from"
  default     = "Altinn"
}

variable "github_org_id" {
  type        = string
  description = "Immutable numeric id of the Github org, used in the immutable OIDC subject claim"
}

variable "github_environments" {
  type        = set(string)
  description = "Github action environments with matching federation"

  validation {
    # Keep this expression in sync with the slug maps in azure-identity.tf.
    # Flattening is not one-to-one - fix/dis and fix_dis both become fix_dis - and two
    # entries that collide would yield two resources sharing one Azure credential name.
    # ARM upserts the second over the first, leaving a plan that can never converge.
    condition = length(distinct([
      for v in var.github_environments : replace(v, "/[^a-zA-Z0-9_-]/", "_")
    ])) == length(var.github_environments)
    error_message = "Values must stay distinct after flattening for the Azure credential name; got ${join(", ", sort(var.github_environments))}."
  }
}

variable "github_branches" {
  type        = set(string)
  description = "Github branches with matching federation"

  validation {
    # Keep this expression in sync with the slug maps in azure-identity.tf.
    # Flattening is not one-to-one - fix/dis and fix_dis both become fix_dis - and two
    # entries that collide would yield two resources sharing one Azure credential name.
    # ARM upserts the second over the first, leaving a plan that can never converge.
    condition = length(distinct([
      for v in var.github_branches : replace(v, "/[^a-zA-Z0-9_-]/", "_")
    ])) == length(var.github_branches)
    error_message = "Values must stay distinct after flattening for the Azure credential name; got ${join(", ", sort(var.github_branches))}."
  }
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
