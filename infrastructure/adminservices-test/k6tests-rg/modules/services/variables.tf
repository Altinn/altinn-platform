variable "k8s_rbac" {
  type = map(
    object(
      {
        namespace = string
        dev_group = string
        sp_group  = string
      }
    )
  )
}

variable "oidc_issuer_url" {
  # Output from azurerm_kubernetes_cluster.k6tests.oidc_issuer_url
}

variable "remote_write_endpoint" {
  # TODO: Last time I didn't find an easy way to get this. Dedicate some time to this again.
  # "https://k6tests-amw-0vej.norwayeast-1.metrics.ingest.monitor.azure.com/dataCollectionRules/dcr-81e9cf1b38fb4648b047399c5593ebda/streams/Microsoft-PrometheusMetrics/api/v1/write?api-version=2023-04-24"
}
