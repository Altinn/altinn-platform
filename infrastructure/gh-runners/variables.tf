variable "subscription_id" {
  type        = string
  description = "Azure subscription ID"
}

variable "altinn_app_id" {
  type        = string
  description = "GitHub App ID for Altinn"
}

variable "altinn_app_install_id" {
  type        = string
  description = "GitHub App installation ID for Altinn"
}

variable "altinn_app_key" {
  type        = string
  description = "GitHub App private key for Altinn"
  sensitive   = true
}

variable "host_ip" {
  type        = string
  description = "Host IP address for Key Vault IP rules"
}

variable "container_apps_managers" {
  type        = list(string)
  description = "Object IDs of users and groups allowed to manage deployed Container Apps infrastructure"
  default     = []
}

variable "terraform_reader_principal_id" {
  type        = string
  description = "Object ID of the service principal used for Terraform plan (reader environment)"
}
