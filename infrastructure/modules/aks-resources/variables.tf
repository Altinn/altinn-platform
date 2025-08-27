variable "subscription_id" {
  type        = string
  description = "Subscription id where aks cluster and other resources are deployed"
}

variable "aks_node_resource_group" {
  type        = string
  description = "AKS node resource group name"
}

variable "azurerm_kubernetes_cluster_id" {
  type        = string
  description = "AKS cluster resource id"
}

variable "environment" {
  type        = string
  description = "Environment"
  validation {
    condition     = length(var.environment) > 0
    error_message = "You must provide a value for environment."
  }
}

variable "flux_release_tag" {
  type        = string
  default     = "latest"
  description = "OCI image that Flux should watch and reconcile"
}

variable "grafana_dashboard_release_branch" {
  type        = string
  default     = "release"
  description = "Grafana dashboard release branch"
  validation {
    condition     = can(regex("^[-A-Za-z0-9_/\\.]+$", var.grafana_dashboard_release_branch))
    error_message = "grafana_dashboard_release_branch must be a valid Git branch (alphanumeric, '-', '_', '/', or '.')."
  }
}

variable "enable_grafana_operator" {
  type        = bool
  description = "Toggle deployment of grafana operator in cluster. If deployed grafana_endpoint must be defined"
  default     = true
}

variable "grafana_endpoint" {
  type        = string
  description = "URL endpoint for Grafana dashboard access"
  default     = ""
  validation {
    condition     = var.enable_grafana_operator == false || (var.enable_grafana_operator == true && length(var.grafana_endpoint) > 0)
    error_message = "You must provide a value for grafana_endpoint when enable_grafana_operator is true."
  }
}

variable "obs_client_id" {
  type        = string
  description = "Client id for the obs app"
}

variable "obs_kv_uri" {
  type        = string
  description = "Key vault uri for observability"
}

variable "obs_tenant_id" {
  type        = string
  description = "Tenant id for the obs app"
}

variable "pip4_ip_address" {
  type        = string
  description = "AKS ipv4 public ip"
}

variable "pip6_ip_address" {
  type        = string
  description = "AKS ipv6 public ip"
}

variable "subnet_address_prefixes" {
  type = object({
    aks_syspool  = list(string)
    aks_workpool = list(string)
  })
  description = "list of subnets"
}

variable "syncroot_namespace" {
  type        = string
  description = "The namespace to use for the syncroot. This is the containing 'folder' in altinncr repo and the namespace in the cluster."
  validation {
    condition     = length(var.syncroot_namespace) > 0
    error_message = "You must provide a value for syncroot_namespace."
  }
}

variable "token_grafana_operator" {
  type        = string
  sensitive   = true
  description = "Authentication token for Grafana operator to manage Grafana resources"
  default     = ""
}

variable "enable_dis_identity_operator" {
  type        = bool
  default     = false
  description = "Enable the dis-identity-operator to manage User Assigned Managed Identities in the cluster."

}

variable "azurerm_kubernetes_cluster_oidc_issuer_url" {
  type        = string
  description = "The OIDC issuer URL of the AKS cluster."
  default     = ""
  validation {
    condition     = var.enable_dis_identity_operator == false || (var.enable_dis_identity_operator == true && length(var.azurerm_kubernetes_cluster_oidc_issuer_url) > 0)
    error_message = "You must provide a value for azurerm_kubernetes_cluster_oidc_issuer_url when enable_dis_identity_operator is true."
  }
}

variable "azurerm_dis_identity_resource_group_id" {
  type        = string
  description = "The resource group ID where the User Assigned Managed Identity managed by dis-identity-operator will be created."
  default     = ""
  validation {
    condition     = var.enable_dis_identity_operator == false || (var.enable_dis_identity_operator == true && length(var.azurerm_dis_identity_resource_group_id) > 0)
    error_message = "You must provide a value for azurerm_dis_identity_resource_group_id when enable_dis_identity_operator is true."
  }
}

variable "enable_cert_manager_tls_issuer" {
  type        = bool
  default     = true
  description = "Enable cert-manager issuer for TLS certificates"
}

variable "tls_cert_manager_workload_identity_client_id" {
  type        = string
  description = "Client id for cert-manager workload identity"
  default     = ""
  validation {
    condition     = var.enable_cert_manager_tls_issuer == false || (var.enable_cert_manager_tls_issuer == true && length(var.tls_cert_manager_workload_identity_client_id) > 0)
    error_message = "You must provide a value for tls_cert_manager_workload_identity_client_id when enable_cert_manager_tls_issuer is true."
  }
}

variable "tls_cert_manager_zone_name" {
  type        = string
  description = "Azure DNS zone name for TLS certificates"
  default     = ""
  validation {
    condition     = var.enable_cert_manager_tls_issuer == false || (var.enable_cert_manager_tls_issuer == true && length(var.tls_cert_manager_zone_name) > 0)
    error_message = "You must provide a value for tls_cert_manager_zone_name when enable_cert_manager_tls_issuer is true."
  }
}

variable "tls_cert_manager_zone_rg_name" {
  type        = string
  description = "Azure DNS zone resource group name for TLS certificates"
  default     = ""
  validation {
    condition     = var.enable_cert_manager_tls_issuer == false || (var.enable_cert_manager_tls_issuer == true && length(var.tls_cert_manager_zone_rg_name) > 0)
    error_message = "You must provide a value for tls_cert_manager_zone_rg_name when enable_cert_manager_tls_issuer is true."
  }
}
