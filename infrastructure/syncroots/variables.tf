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
    refs         = set(string)
  }))
}