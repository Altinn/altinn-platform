resource "azapi_resource" "flux_syncroot" {
  type      = "Microsoft.KubernetesConfiguration/fluxConfigurations@2024-11-01"
  name      = "syncroot-${var.syncroot_namespace}-${var.environment}"
  parent_id = var.azurerm_kubernetes_cluster_id
  body = {
    properties = {
      kustomizations = {
        syncroot = {
          force                  = false
          path                   = "./${var.environment}"
          prune                  = false
          retryIntervalInSeconds = 300
          syncIntervalInSeconds  = 300
          timeoutInSeconds       = 600
          wait                   = true
        }
      }
      ociRepository = {
        insecure = false
        repositoryRef = {
          tag = var.environment
        }
        syncIntervalInSeconds = 300
        timeoutInSeconds      = 300
        url                   = "oci://altinncr.azurecr.io/${var.syncroot_namespace}/syncroot"
        useWorkloadIdentity   = true
      }
      namespace                  = var.syncroot_namespace
      reconciliationWaitDuration = "PT5M"
      waitForReconciliation      = false
      sourceKind                 = "OCIRepository"
    }
  }
}
