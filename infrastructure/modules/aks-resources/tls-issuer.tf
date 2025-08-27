resource "azapi_resource" "cert_manager_issuer" {
  count = var.enable_cert_manager_tls_issuer ? 1 : 0
  depends_on = [azapi_resource.cert_manager]
  type       = "Microsoft.KubernetesConfiguration/fluxConfigurations@2025-04-01"
  name       = "tls-issuer"
  parent_id  = var.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        tls-issuer = {
          force = false
          path  = "./"
          postBuild = {
            substitute = {
              AZURE_DNS_ZONE_NAME   = "${var.tls_cert_manager_zone_name}"
              AZURE_RESOURCE_GROUP  = "${var.tls_cert_manager_zone_rg_name}"
              AZURE_SUBSCRIPTION_ID = "${var.subscription_id}"
              IDENTITY_CLIENT_ID    = "${var.tls_cert_manager_workload_identity_client_id}"
            }
          }
          prune                  = false
          retryIntervalInSeconds = 300
          syncIntervalInSeconds  = 300
          timeoutInSeconds       = 300
          wait                   = true
        }
      }
      ociRepository = {
        insecure = false
        repositoryRef = {
          tag = var.flux_release_tag
        }
        syncIntervalInSeconds = 300
        timeoutInSeconds      = 300
        url                   = "oci://altinncr.azurecr.io/manifests/infra/certm-lets-encrypt-dns-issuer"
        useWorkloadIdentity   = true
      }
      namespace                  = "flux-system"
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = true
      sourceKind                 = "OCIRepository"
    }
  }
}
