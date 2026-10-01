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
    condition = alltrue([
      for repo_name in distinct([for repo in var.product_syncroot_source_repos : repo.repo_name]) :
      length([for repo in var.product_syncroot_source_repos : repo if repo.repo_name == repo_name]) == 1 ||
      contains(keys(var.repo_identity_products), repo_name)
    ])
    error_message = "Repositories shared by multiple products must select an existing product in repo_identity_products to keep a stable publishing identity."
  }

  validation {
    condition = alltrue(flatten([
      for repo in var.product_syncroot_source_repos : [
        for other in var.product_syncroot_source_repos :
        repo.repo_name != other.repo_name || (repo.branches == other.branches && repo.environments == other.environments)
      ]
    ]))
    error_message = "Products sharing a repository must have identical branches and environments because they share one publishing identity."
  }

  validation {
    condition = alltrue([
      for repo_name, product in var.repo_identity_products :
      try(var.product_syncroot_source_repos[product].repo_name == repo_name, false)
    ])
    error_message = "Each repo_identity_products value must name a configured product belonging to that repository."
  }
}

variable "repo_identity_products" {
  type        = map(string)
  description = "Repository to product whose publishing identity and Terraform address are retained when several products share the repository."
  default     = {}
}
