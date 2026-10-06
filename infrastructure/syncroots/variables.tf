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
    condition     = alltrue([for k, v in var.product_syncroot_source_repos : can(regex("^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$", k))])
    error_message = "Product names (map keys) must contain alphanumeric segments separated by single hyphens."
  }

  validation {
    condition = length(distinct([
      for repo in var.product_syncroot_source_repos : lower(replace(repo.repo_name, ".", "_"))
    ])) == length(distinct([for repo in var.product_syncroot_source_repos : repo.repo_name]))
    error_message = "Repository names must remain unique, ignoring case, after replacing periods with underscores for Azure identity names."
  }

  validation {
    condition = alltrue(flatten([
      for repo in var.product_syncroot_source_repos : [
        for other in var.product_syncroot_source_repos :
        repo.repo_name != other.repo_name || repo.branches == other.branches
      ]
    ]))
    error_message = "Products sharing a repository must have identical branches because they share one publishing identity."
  }
}
