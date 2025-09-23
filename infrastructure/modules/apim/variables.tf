variable "apim_rg_name" {
  type        = string
  description = "The name of the Resource Group in which the API Management service should be created. If not specified, a name will be generated."
  default     = ""
}

variable "prefix" {
  type        = string
  description = "Resources prefixes"
  validation {
    condition     = can(regex("^[a-zA-Z][a-zA-Z0-9]{0,31}$", var.prefix))
    error_message = "The 'prefix' variable must start with a letter, contain only alphanumeric characters, and be between 1 and 32 characters long."
  }
}

variable "environment" {
  type        = string
  description = "The deployment environment name (e.g., at22, dev, tt02, test, prod)."
  validation {
    condition     = length(var.environment) > 0 && length(var.environment) <= 5
    error_message = "The 'environment' variable must be between 1 and 5 characters long."
  }
}

variable "location" {
  type        = string
  description = "The Azure region where the resources will be deployed."
  default     = "norwayeast"
}

variable "publisher" {
  type        = string
  description = "The name of the publisher for the API Management service."
  default     = "Altinn"
}

variable "publisher_email" {
  type        = string
  description = "The email address of the publisher for the API Management service."
  validation {
    condition     = can(regex("^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$", var.publisher_email))
    error_message = "The 'publisher_email' variable must be a valid email address."
  }
}

variable "sku_name" {
  type        = string
  description = "SKU name"
  default     = "Developer_1"
  validation {
    condition     = can(regex("^(Consumption|Developer|Basic|BasicV2|Standard|StandardV2|Premium|PremiumV2)(_[0-9]+)?$", var.sku_name))
    error_message = "Invalid sku_name. Valid values are: Consumption, Developer_1, Basic_1, BasicV2_1, Standard_1, StandardV2_1, Premium_1, PremiumV2_1. With the number of scale units at the end for non-Consumption SKUs. e.g. Developer_1"
  }
}

variable "sampling_percentage" {
  type        = number
  description = "Sampling percentage for Application Insights diagnostics. Set to 0.0 to log only errors."
  default     = 0.0
  validation {
    condition     = var.sampling_percentage >= 0.0 && var.sampling_percentage <= 100.0
    error_message = "The 'sampling_percentage' variable must be between 0.0 and 100.0."
  }
}

variable "headers_to_log" {
  type        = set(string)
  description = "A set of headers to log in Application Insights diagnostics. By default, no headers are logged."
  default     = []
}

variable "body_bytes_to_log" {
  type        = number
  default     = 0
  description = "The number of payload bytes (up to 8192) to log in Application Insights diagnostics. By default, no payload is logged."
  validation {
    condition     = var.body_bytes_to_log >= 0 && var.body_bytes_to_log <= 8192
    error_message = "The 'body_bytes_to_log' variable must be a number between 0 and 8192."
  }
}

variable "tags" {
  type        = map(string)
  description = "A map of tags to assign to the created resources."
  default     = {}
}

variable "apim_service_contributors" {
  type        = map(string)
  default     = {}
  description = "A map of principal IDs to grant the 'API Management Service Contributor' role. The map key is a descriptive name for the assignment, and the value is the principal's object ID."
}