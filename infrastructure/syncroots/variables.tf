variable "subscription_id" {
  type        = string
  description = "subscription id where uamis are deployed"
}

variable "github_org_name" {
  type        = string
  description = "Github organization name"
}

variable "product_syncroot_source_repos" {
  type = map(object({
    repo_name    = string
    environments = set(string)
    branches     = set(string)
  }))

  validation {
    condition     = alltrue([for k, v in var.product_syncroot_source_repos : can(regex("^[a-zA-Z0-9]+$", k))])
    error_message = "Product names (map keys) must be alphanumeric characters only."
  }
}