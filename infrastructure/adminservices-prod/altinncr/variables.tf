variable "subscription_id" {
  type = string
}

variable "acr_rgname" {
  type        = string
  description = "Name acr resource group"
}

variable "acrname" {
  type        = string
  description = "Name on container registry"
}

variable "cache_rules" {
  type = list(object({
    name              = string
    target_repo       = string
    source_repo       = string
    credential_set_id = string
  }))
}

variable "acr_push_object_ids" {
  type        = set(string)
  description = "object_ids that should be granted AcrPush role on the container registry"
  default     = []
}
